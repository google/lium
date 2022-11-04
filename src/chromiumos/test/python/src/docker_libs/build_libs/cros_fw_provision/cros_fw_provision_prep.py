# Copyright 2022 The ChromiumOS Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Docker build Context prep for cros-fw-provision."""

import sys


# Point up a few directories to make the other python modules discoverable.
sys.path.append("../../../../")

from src.docker_libs.build_libs.cros_fw_provision.container_prep import (  # noqa: E402 pylint: disable=import-error,wrong-import-position
    CrosFWProvisionArtifactPrep,
)
from src.docker_libs.build_libs.shared.base_prep import (  # noqa: E402 pylint: disable=import-error,wrong-import-position
    BaseDockerPrepper,
)


class CrosFWProvisionDockerPrepper(BaseDockerPrepper):
    """Prep Needed files for the Test Execution Container Docker Build."""

    def __init__(self, chroot: str, sysroot: str, tags: str, labels: str, service: str):
        """@param args (ArgumentParser): .chroot, .sysroot, .path."""
        super().__init__(
            chroot=chroot, sysroot=sysroot, tags=tags, labels=labels, service=service
        )

    def prep_container(self):
        CrosFWProvisionArtifactPrep(
            chroot=self.chroot,
            sysroot=self.sysroot,
            path=self.full_out_dir,
            force_path=True,
        ).prep()