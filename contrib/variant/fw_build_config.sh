#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

VERSION="1.0.1"
SCRIPT=$(basename -- "${0}")
set -e

export LC_ALL=C

if [[ ! -e /etc/cros_chroot_version ]]; then
  echo "This script must be run inside the chroot."
  exit 1
fi

if [[ "$#" -lt 2 ]]; then
  echo "Usage: ${SCRIPT} base_name variant_name [bug_number]"
  echo "e.g. ${SCRIPT} puff wyvern b:158269582"
  echo "Updates the config.star to add a default _FW_BUILD_CONFIG"
  exit 1
fi

# This is the name of the base board.
# ${var,,} converts to all lowercase.
BASE="${1,,}"
# This is the name of the variant that we are enabling _FW_BUILD_CONFIG for.
VARIANT="${2,,}"
# We need all uppercase version, too, so ${var^^}
VARIANT_UPPER="${VARIANT^^}"

# Assign BUG= text, or "None" if that parameter wasn't specified.
BUG="${3:-None}"

# The config.star file will be located here
cd "${HOME}/trunk/src/project/${BASE}/${VARIANT}"

# Start a branch. Use YMD timestamp to avoid collisions.
DATE=$(date +%Y%m%d)
BRANCH="create_${VARIANT}_${DATE}"
repo start "${BRANCH}" .

function catch() {
  # If there is an error after the `repo start`, then restore modified files
  # to clean up. `git restore --staged` will remove files added to a commit,
  # and `git restore` will restore the file to its unmodified state. Then
  # `repo abandon` the new branch.
  git restore --staged config.star
  git restore config.star
  git restore --staged generated/config.jsonproto
  git restore generated/config.jsonproto
  git restore --staged sw_build_config/platform/chromeos-config/generated/project-config.json
  git restore sw_build_config/platform/chromeos-config/generated/project-config.json
  repo abandon "$1" .
}
trap 'catch "${BRANCH}"' ERR

# Change the _FW_BUILD_CONFIG from None to _${VARIANT_UPPER}.
sed -i -e "s/_FW_BUILD_CONFIG = None/_FW_BUILD_CONFIG = program.firmware_build_config(_${VARIANT_UPPER})/" config.star

# Regenerate the config.
./config.star

# Add modified files.
git add config.star
git add generated/config.jsonproto
git add sw_build_config/platform/chromeos-config/generated/project-config.json

# Now commit the files.
git commit -m "${VARIANT}: enable default firmware build

Add a default _FW_BUILD_CONFIG.

(Auto-Generated by ${SCRIPT} version ${VERSION}).

BUG=${BUG}
TEST=Verify the ${VARIANT} firmware builds"
