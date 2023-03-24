#!/bin/bash
# Copyright 2020 The ChromiumOS Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

VERSION="2.0.0"
SCRIPT=$(basename -- "${0}")
set -e

export LC_ALL=C

if [[ ! -e /etc/cros_chroot_version ]]; then
  echo "This script must be run inside the chroot."
  exit 1
fi

if [[ "$#" -lt 2 ]]; then
  echo "Usage: ${SCRIPT} base_name variant_name [bug_number]"
  echo "e.g. ${SCRIPT} hatch kohaku b:140261109"
  echo "Adds a new device and unprovisioned SKU to the YAML file for"
  echo "the variant being created. Revbump the ebuild."
  exit 1
fi

# shellcheck source=revbump_ebuild.sh
# shellcheck disable=SC1091
source "${BASH_SOURCE%/*}/revbump_ebuild.sh"

# shellcheck source=check_standalone.sh
# shellcheck disable=SC1091
source "${BASH_SOURCE%/*}/check_standalone.sh"
check_standalone

# shellcheck source=check_pending_changes.sh
# shellcheck disable=SC1091
source "${BASH_SOURCE%/*}/check_pending_changes.sh"

# This is the name of the base board that we're using to make the variant.
# ${var,,} converts to all lowercase.
BASE="${1,,}"
# This is the name of the variant that is being cloned.
VARIANT="${2,,}"
# ${var^} capitalizes first letter only.
VARIANT_CAPITALIZED="${VARIANT^}"
# We need all uppercase version, too, so ${var^^}
BASE_UPPER="${BASE^^}"

# Assign BUG= text, or "None" if that parameter wasn't specified.
BUG=${3:-None}

YAML=model.yaml

cd "${HOME}/trunk/src/overlays/overlay-${BASE}/chromeos-base/chromeos-config-bsp/files"

if [[ ! -e "${YAML}" ]]; then
  echo "${YAML} does not exist."
  exit 1
fi

# Make sure the variant doesn't already exist in the yaml file.
if grep -qi "${VARIANT}" "${YAML}" ; then
  echo "${VARIANT} already appears to exist in ${YAML}"
  echo "Have you already created this variant?"
  exit 1
fi

# If there are pending changes, exit the script (unless overridden)
check_pending_changes "$(pwd)"

# Start a branch. Use YMD timestamp to avoid collisions.
DATE=$(date +%Y%m%d)
BRANCH="create_${VARIANT}_${DATE}"
repo start "${BRANCH}" . "${NEW_VARIANT_WIP:+--head}"
# ${parameter:+word}" substitutes "word" if $parameter is set to a non-null
# value, or substitutes null if $parameter is null or unset.

cleanup() {
  # If there is an error after the `repo start`, then restore modified files
  # to clean up and `repo abandon` the new branch.
  cd "${HOME}/trunk/src/overlays/overlay-${BASE}/chromeos-base/chromeos-config-bsp"
  git restore --staged "*.ebuild"
  git restore "*.ebuild"
  if [[ ! -z "${NEWEBUILD}" ]] ; then
    rm -f "${NEWEBUILD}"
  fi
  cd files
  git restore --staged "${YAML}"
  git restore "${YAML}"
  repo abandon "${BRANCH}" .
}
trap 'cleanup' ERR

# ebuild is located 1 directory up.
pushd ..
revbump_ebuild
popd

# Append a new device-name to the end of the model.yaml file.
cat <<EOF >>"${YAML}"
    - \$device-name: "unprovisioned_${VARIANT}"
      \$fw-name: "${VARIANT_CAPITALIZED}"
      products:
        - \$key-id: "${BASE_UPPER}"
      skus:
        - \$sku-id: 255
          config: *base_config
EOF
git add "${YAML}"
# Building the config.json and other files from the yaml requires that the
# changes have been made to both the public and private yaml files, which
# we can't guarantee here, so we will not automate the steps in the TEST=
# part of the commit message below. The high-level program which calls this
# script and others will be responsible for testing the emerge command and
# verifying that the new variant shows up in the output.

# Now commit the files.
git commit -m "model.yaml: Add ${VARIANT} variant

Add a new section to the yaml file to define the ${VARIANT}
variant of the ${BASE} baseboard.

(Auto-Generated by ${SCRIPT} version ${VERSION}).

BUG=${BUG}
TEST=emerge-${BASE} chromeos-config-bsp-hatch
chromeos-config-bsp-private chromeos-config-bsp chromeos-config
 Check /build/${BASE}/usr/share/chromeos-config for '${VARIANT}' in
 config.json, yaml/config.c, and yaml/*.yaml"