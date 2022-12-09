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

type CreateBinaryDirsCommand struct {
	ctx context.Context
	cs  *service.AShService
}

func NewCreateBinaryDirsCommand(ctx context.Context, cs *service.AShService) *CreateBinaryDirsCommand {
	return &CreateBinaryDirsCommand{
		ctx: ctx,
		cs:  cs,
	}
}

func (c *CreateBinaryDirsCommand) Execute(log *log.Logger) error {
	return c.cs.Connection.CreateDirectories(c.ctx, []string{c.cs.GetTargetDir(), c.cs.GetAutotestDir(), c.cs.GetTastDir()})
}

func (c *CreateBinaryDirsCommand) Revert() error {
	return nil
}

func (c *CreateBinaryDirsCommand) GetErrorMessage() string {
	return "failed to create binary directories"
}

func (c *CreateBinaryDirsCommand) GetStatus() api.InstallResponse_Status {
	return api.InstallResponse_STATUS_PROVISIONING_FAILED
}
