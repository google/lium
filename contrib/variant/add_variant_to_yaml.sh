#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

VERSION="1.2.0"
SCRIPT=$(basename -- "${0}")

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

if [[ "${BASE}" == "volteer" ]]; then
  echo "Volteer does not need changes to public yaml, exiting ..."
  exit 0
fi

if [[ "${BASE}" == "zork" ]]; then
  echo "Zork does not need changes to public yaml, exiting ..."
  exit 0
fi

# Can't put the ~ inside the "" but I need the "" to avoid spaces and globbing
# for ${BASE}, so it's two separate commands.
cd ~ || exit 1
cd "trunk/src/overlays/overlay-${BASE}/chromeos-base/chromeos-config-bsp-${BASE}/files" || exit 1

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

# Start a branch. Use YMD timestamp to avoid collisions.
DATE=$(date +%Y%m%d)
repo start "create_${VARIANT}_${DATE}" . || exit 1

# We need to revbump the ebuild file. It has the version number in
# its name, and furthermore, it's a symlink to another ebuild file.
#
# Find a symlink named *.ebuild, should be only one.
EBUILD=$(find .. -name "*.ebuild" -type l)
# Remove the extension
F=${EBUILD%.ebuild}
# Get the numeric suffix after the 'r'.
# If $F == ./coreboot-private-files-hatch-0.0.1-r30
# then we want '30'.
# We need to reverse the string because cut only supports cutting specific
# fields from the start a string (you can't say N-1, N-2 in general) and
# we need the last fields.
REVISION=$(echo "${F}" | rev | cut -d- -f 1 | cut -dr -f 1 | rev)
# Incremement
NEWREV=$((REVISION + 1))
# Replace e.g. 'r30' with 'r31' in the file name
NEWEBUILD="${EBUILD/r${REVISION}.ebuild/r${NEWREV}.ebuild}"
# Rename
git mv "${EBUILD}" "${NEWEBUILD}"

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
TEST=emerge-${BASE} chromeos-config-bsp-${BASE}
chromeos-config-bsp-${BASE}-private chromeos-config-bsp chromeos-config
 Check /build/${BASE}/usr/share/chromeos-config for '${VARIANT}' in
 config.json, yaml/config.c, and yaml/*.yaml"
