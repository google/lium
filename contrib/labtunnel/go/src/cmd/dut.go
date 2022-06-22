// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cmd

import (
	"time"

	"chromiumos/platform/dev/contrib/labtunnel/log"
	"github.com/spf13/cobra"
)

var (
	dutCmd = &cobra.Command{
		Use:   "dut <dut_hostname>",
		Short: "Ssh tunnel to dut.",
		Long: `
Opens an ssh tunnel to the remote ssh port to the dut as defined by
dut_hostname.

All tunnels are destroyed upon stopping labtunnel, and are restarted if
interrupted by a remote device reboot.

The dut hostname is resolved from <dut_hostname> by removing the prefix
"crossk-" if it is present.
`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sshManager := buildSshManager()

			// Tunnel to dut.
			hostDut := resolveHostname(args[0], "")
			localDut := tunnelLocalPortToRemotePort(cmd.Context(), sshManager, "DUT", "", remotePortSsh, hostDut)

			time.Sleep(time.Second)
			log.Logger.Printf("Example Tast call (in chroot): tast run %s <test>", localDut)
			sshManager.WaitUntilAllSshCompleted(cmd.Context())
		},
	}
)

func init() {
	rootCmd.AddCommand(dutCmd)
}
