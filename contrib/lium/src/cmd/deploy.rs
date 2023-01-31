// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

use anyhow::Result;
use argh::FromArgs;
use lium::chroot::Chroot;
use lium::cros::ensure_testing_rsa_is_there;
use lium::dut::SshInfo;
use lium::repo::get_repo_dir;
use regex_macro::regex;

#[derive(FromArgs, PartialEq, Debug)]
/// cros deploy wrapper
#[argh(subcommand, name = "deploy")]
pub struct Args {
    /// target cros repo dir
    #[argh(option)]
    repo: Option<String>,

    /// a DUT identifier (e.g. 127.0.0.1, localhost:2222)
    #[argh(option)]
    dut: String,

    /// package to build
    #[argh(option)]
    package: String,

    /// if specified, it will invoke autologin
    #[argh(switch)]
    autologin: bool,
}
pub fn run(args: &Args) -> Result<()> {
    ensure_testing_rsa_is_there()?;
    let target = SshInfo::new(&args.dut)?;
    println!("Target DUT is {:?}", target);
    let board = target.get_board()?;
    let package = &args.package;
    let re_cros_kernel = regex!(r"chromeos-kernel-");
    let target = SshInfo::new(&args.dut)?;
    let mut ssh_forwarding_control: Option<async_process::Child> = None;
    let target = if target.needs_port_forwarding_in_chroot() {
        ssh_forwarding_control = Some(target.start_ssh_forwarding(2222)?);
        SshInfo::new("localhost:2222")?
    } else {
        target
    };
    let chroot = Chroot::new(&get_repo_dir(&args.repo)?)?;
    if re_cros_kernel.is_match(package) {
        chroot.run_bash_script_in_chroot(
            "update_kernel",
            &format!(
                r###"
cros-workon-{board} start {package}
~/trunk/src/scripts/update_kernel.sh --remote={} --ssh_port {} --remote_bootargs
"###,
                target.host(),
                target.port()
            ),
            None,
        )?;
    } else {
        chroot.run_bash_script_in_chroot(
            "deploy",
            &format!(
                r"cros-workon-{board} start {package} && cros deploy {} {package}",
                target.host_and_port()
            ),
            None,
        )?;
    }
    if args.autologin {
        target.run_autologin()?;
    }
    drop(ssh_forwarding_control);
    Ok(())
}
