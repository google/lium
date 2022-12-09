// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"chromiumos/test/provision/v2/ash-provision/service"
	"context"
	"log"

	"go.chromium.org/chromiumos/config/go/test/api"
)

type MountRootFSCommand struct {
	ctx context.Context
	cs  *service.AShService
}

func NewMountRootFSCommand(ctx context.Context, cs *service.AShService) *MountRootFSCommand {
	return &MountRootFSCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *MountRootFSCommand) Execute(log *log.Logger) error {
	if _, err := c.cs.Connection.RunCmd(c.ctx, "mount", []string{"-o", "remount,rw", "/"}); err != nil {
		return err
	}
	return nil
}

func (c *MountRootFSCommand) Revert() error {
	return nil
}

func (c *MountRootFSCommand) GetErrorMessage() string {
	return "failed to mount root file system"
}

func (c *MountRootFSCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
