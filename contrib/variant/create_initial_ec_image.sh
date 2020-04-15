#!/bin/bash
# Copyright 2019 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

VERSION="1.0.2"
SCRIPT=$(basename -- "${0}")

export LC_ALL=C

if [[ ! -e /etc/cros_chroot_version ]]; then
  echo "This script must be run inside the chroot."
  exit 1
fi

if [[ "$#" -lt 2 ]]; then
  echo "Usage: ${SCRIPT} reference_name variant_name [bug_number]"
  echo "e.g. ${SCRIPT} hatch kohaku b:140261109"
  echo "Creates the initial EC image as a copy of the reference board's EC."
  exit 1
fi

# This is the name of the reference board that we copying to make the variant.
# ${var,,} converts to all lowercase.
REF="${1,,}"
# This is the name of the variant that is being cloned.
VARIANT="${2,,}"

# Assign BUG= text, or "None" if that parameter wasn't specified.
BUG=${3:-None}

# All of the necessary files are in platform/ec/board
cd ~/trunk/src/platform/ec/board || exit 1

# Make sure that the reference board exists.
if [[ ! -e "${REF}" ]]; then
  echo "${REF} does not exist; please specify a valid reference board."
  exit 1
fi

# Make sure the variant doesn't already exist.
if [[ -e "${VARIANT}" ]]; then
  echo "${VARIANT} already exists; have you already created this variant?"
  exit 1
fi

# Start a branch. Use YMD timestamp to avoid collisions.
DATE=$(date +%Y%m%d)
repo start "create_${VARIANT}_${DATE}" . || exit 1

mkdir "${VARIANT}"
cp "${REF}"/* "${VARIANT}"

# Update copyright notice to current year.
YEAR=$(date +%Y)
find "${VARIANT}" -type f -exec \
    sed -i -e "s/Copyright.*20[0-9][0-9]/Copyright ${YEAR}/" {} +

# Build the code; exit if it fails.
pushd .. || exit 1
make -j BOARD="${VARIANT}" || exit 1
popd || exit 1

git add "${VARIANT}"/*

# Now commit the files. Use fmt to word-wrap.
MSG=$(echo "${VARIANT}: Initial EC image

Create the initial EC image for the ${VARIANT} variant
by copying the ${REF} reference board EC files into a new
directory named for the variant.

(Auto-Generated by ${SCRIPT} version ${VERSION}).

BUG=${BUG}
BRANCH=none
TEST=make BOARD=${VARIANT}" | fmt -w 70)
git commit -sm "${MSG}"
