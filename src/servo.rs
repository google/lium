// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

use crate::chroot::Chroot;
use crate::util::gen_path_in_lium_dir;
use crate::util::get_async_lines;
use crate::util::get_stdout;
use crate::util::require_root_privilege;
use crate::util::run_bash_command;
use anyhow::anyhow;
use anyhow::Context;
use anyhow::Result;
use async_process::Child;
use futures::executor::block_on;
use futures::select;
use futures::FutureExt;
use futures::StreamExt;
use rand::seq::SliceRandom;
use rand::thread_rng;
use serde::Deserialize;
use serde::Serialize;
use std::collections::HashMap;
use std::collections::HashSet;
use std::fmt;
use std::fmt::Display;
use std::fmt::Formatter;
use std::fs;
use std::fs::read_to_string;
use std::fs::write;
use std::iter::FromIterator;
use std::path::Path;

fn read_usb_attribute(dir: &Path, name: &str) -> Result<String> {
    let value = dir.join(name);
    let value = fs::read_to_string(value)?;
    Ok(value.trim().to_string())
}

// This is private since users should use ServoList instead
fn discover() -> Result<Vec<LocalServo>> {
    let paths = fs::read_dir("/sys/bus/usb/devices/").unwrap();
    Ok(paths
        .flat_map(|usb_path| -> Result<LocalServo> {
            let usb_sysfs_path = usb_path?.path();
            let product = read_usb_attribute(&usb_sysfs_path, "product")?;
            let serial = read_usb_attribute(&usb_sysfs_path, "serial")?;
            if product.starts_with("Servo")
                || product.starts_with("Cr50")
                || product.starts_with("Ti50")
            {
                let paths = fs::read_dir(&usb_sysfs_path).context("failed to read dir")?;
                let tty_list: HashMap<String, String> = paths
                    .flat_map(|path| -> Result<(String, String)> {
                        let path = path?.path();
                        let interface = fs::read_to_string(path.join("interface"))?
                            .trim()
                            .to_string();
                        let tty_name = fs::read_dir(path)?
                            .find_map(|p| {
                                let s = p.ok()?.path();
                                let s = s.file_name()?.to_string_lossy().to_string();
                                s.starts_with("ttyUSB").then_some(s.clone())
                            })
                            .context("ttyUSB not found")?;
                        Ok((interface, tty_name))
                    })
                    .collect();
                Ok(LocalServo {
                    product,
                    serial,
                    usb_sysfs_path: usb_sysfs_path.to_string_lossy().to_string(),
                    tty_list,
                    ..Default::default()
                })
            } else {
                Err(anyhow!("Not a servo"))
            }
        })
        .collect())
}

fn discover_slow() -> Result<Vec<LocalServo>> {
    require_root_privilege()?;
    for mut s in discover()? {
        s.reset().context("Failed to reset servo")?
    }
    let mut servos = discover()?;
    servos.iter_mut().for_each(|s| {
            eprintln!("Checking {}", s.serial);
            let mac_addr = s.tty_list.get("Servo EC Shell").and_then(|id|
                {
                    run_bash_command(&format!("echo macaddr | socat - /dev/{id},echo=0 | grep -E -o '([0-9A-Z]{{2}}:){{5}}([0-9A-Z]{{2}})'"), None)
                        .ok()
                        .filter(|o| {o.status.success()})
                        .as_ref()
                        .map(get_stdout)
                });
            let ec_version = s.tty_list.get("EC").and_then(|id|
                {
                    run_bash_command(&format!("echo version | socat - /dev/{id},echo=0,crtscts=1 | grep 'RO:' | sed -e 's/^RO:\\s*'//"), None)
                        .ok()
                        .filter(|o| {o.status.success()})
                        .as_ref()
                        .map(get_stdout)
                        .filter(|s| {!s.is_empty()})
                });
            s.mac_addr = mac_addr;
            s.ec_version = ec_version;
        });
    Ok(servos)
}

pub fn reset_devices(serials: &Vec<String>) -> Result<()> {
    let servo_info = discover()?;
    let mut servo_info: Vec<LocalServo> = if !serials.is_empty() {
        let serials: HashSet<_> = HashSet::from_iter(serials.iter());
        servo_info
            .iter()
            .filter(|s| serials.contains(&s.serial().to_string()))
            .cloned()
            .collect()
    } else {
        servo_info
    };
    for s in &mut servo_info {
        s.reset()?;
    }
    Ok(())
}

#[derive(Debug, Clone, Deserialize, Serialize, Default)]
pub struct ServoList {
    #[serde(default)]
    devices: Vec<LocalServo>,
}
static CONFIG_FILE_NAME: &str = "servo_list.json";
impl ServoList {
    pub fn read() -> Result<Self> {
        let path = gen_path_in_lium_dir(CONFIG_FILE_NAME)?;
        let list = read_to_string(&path);
        match list {
            Ok(list) => Ok(serde_json::from_str(&list)?),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                // Just create a default config
                let list = Self::default();
                list.write()?;
                eprintln!("INFO: Servo list created at {:?}", path);
                Ok(list)
            }
            e => Err(anyhow!("Failed to create a new servo list: {:?}", e)),
        }
    }
    // This is private since write should happen on every updates transparently
    fn write(&self) -> Result<()> {
        let s = serde_json::to_string_pretty(&self)?;
        write(gen_path_in_lium_dir(CONFIG_FILE_NAME)?, s.into_bytes())
            .context("failed to write servo list")
    }
    pub fn update() -> Result<()> {
        let devices = discover_slow()?;
        let list = Self { devices };
        list.write()
    }
    pub fn find_by_serial(&self, serial: &str) -> Result<&LocalServo> {
        self.devices
            .iter()
            .find(|s| s.serial() == serial)
            .context("Servo not found with a given serial")
    }
    pub fn devices(&self) -> &Vec<LocalServo> {
        &self.devices
    }
}
impl Display for ServoList {
    fn fmt(&self, f: &mut Formatter) -> fmt::Result {
        write!(
            f,
            "{}",
            serde_json::to_string_pretty(&self).map_err(|_| fmt::Error)?
        )
    }
}

#[derive(Debug, Clone, Deserialize, Serialize, Default)]
pub struct LocalServo {
    product: String,
    serial: String,
    usb_sysfs_path: String,
    tty_list: HashMap<String, String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(default)]
    mac_addr: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(default)]
    ec_version: Option<String>,
}
impl LocalServo {
    pub fn product(&self) -> &str {
        &self.product
    }
    pub fn serial(&self) -> &str {
        &self.serial
    }
    pub fn tty_list(&self) -> &HashMap<String, String> {
        &self.tty_list
    }
    pub fn tty_path(&self, tty_type: &str) -> Result<String> {
        let tty_name = self
            .tty_list()
            .get(tty_type)
            .context(anyhow!("tty[{}] not found", tty_type))?;
        Ok(format!("/dev/{tty_name}"))
    }
    pub fn run_cmd(&self, tty_type: &str, cmd: &str) -> Result<String> {
        let tty_path = self.tty_path(tty_type)?;
        let output = run_bash_command(
            &format!("echo {cmd} | socat - {tty_path},echo=0,crtscts=1"),
            None,
        )?;
        output.status.exit_ok()?;
        Ok(get_stdout(&output))
    }
    pub fn reset(&mut self) -> Result<()> {
        eprintln!("Resetting servo device: {}", self.serial);
        let path = Path::new(&self.usb_sysfs_path).join("authorized");
        fs::write(&path, b"0").context("Failed to set authorized = 0")?;
        fs::write(&path, b"1").context("Failed to set authorized = 1")?;
        Ok(())
    }
    pub fn from_serial(serial: &str) -> Result<LocalServo> {
        let servos = discover()?;
        Ok(servos
            .iter()
            .find(|&s| s.serial == serial)
            .context(anyhow!("Servo not found: {serial}"))?
            .clone())
    }
    fn start_servod_on_port(&self, chroot: &Chroot, port: u16) -> Result<Child> {
        chroot
            .exec_in_chroot_async(&[
                "sudo",
                "servod",
                "-s",
                &self.serial,
                "-p",
                &port.to_string(),
            ])
            .context("failed to launch servod")
    }
    pub fn start_servod(&self, chroot: &Chroot) -> Result<ServodConnection> {
        block_on(async {
            eprintln!("Starting servod...");
            let mut ports = (9000..9099).into_iter().collect::<Vec<u16>>();
            let mut rng = thread_rng();
            ports.shuffle(&mut rng);
            for port in ports {
                let mut servod = self.start_servod_on_port(chroot, port)?;
                let (mut servod_stdout, mut servod_stderr) = get_async_lines(&mut servod);
                loop {
                    let mut servod_stdout = servod_stdout.next().fuse();
                    let mut servod_stderr = servod_stderr.next().fuse();
                    select! {
                            line = servod_stderr => {
                                if let Some(line) = line {
                                    let line = line?;
                                eprintln!("{}", line);
                                    if line.contains("is busy") {
                                        break;
                                    }
                                } else {
                    return Err(anyhow!("servod failed unexpectedly"));
                                }
                            }
                            line = servod_stdout => {
                                if let Some(line) = line {
                                    let line = line?;
                                    eprintln!("{}", line);
                                    if line.contains("Listening on localhost port") {
                                        return Result::Ok(servod);
                                    }
                                } else {
                    return Err(anyhow!("servod failed unexpectedly"));
                                }
                            }
                        }
                }
            }
            return Err(anyhow!("servod failed unexpectedly"));
        })?;
        ServodConnection::from_serial(&self.serial)
    }
    pub fn is_cr50(&self) -> bool {
        self.product() == "Cr50" || self.product() == "Ti50"
    }
}

pub struct ServodConnection {
    serial: String,
    host: String,
    port: u16,
}
impl ServodConnection {
    pub fn from_serial(serial: &str) -> Result<Self> {
        let output = run_bash_command(&format!("ps ax | grep /servod | grep -e '-s {}' | grep -E -o -e '-p [0-9]+' | cut -d ' ' -f 2", serial), None);
        if let Ok(output) = output {
            let stdout = get_stdout(&output);
            let port = stdout.parse::<u16>()?;
            Ok(Self {
                serial: serial.to_string(),
                host: "localhost".to_string(),
                port,
            })
        } else {
            Err(anyhow!("Servod for {serial} is not running"))
        }
    }
    pub fn serial(&self) -> &str {
        &self.serial
    }
    pub fn host(&self) -> &str {
        &self.host
    }
    pub fn port(&self) -> u16 {
        self.port
    }
    pub fn run_dut_control<T: AsRef<str>>(&self, chroot: &Chroot, args: &[T]) -> Result<String> {
        eprintln!("Using servod port {:?}", self.port);
        let output = chroot.exec_in_chroot(
            &[
                ["dut-control", "-p", &self.port.to_string()].as_slice(),
                args.iter()
                    .map(AsRef::as_ref)
                    .collect::<Vec<&str>>()
                    .as_slice(),
            ]
            .concat(),
        )?;
        Ok(output)
    }
}