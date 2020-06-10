#!/bin/bash
# Copyright 2020 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e

VERSION="4.0.0"
SCRIPT="$(basename -- "$0")"

if [[ -z "${CB_SRC_DIR}" ]]; then
  echo "CB_SRC_DIR must be set in the environment"
  exit 1
fi

if [[ "$#" -lt 3 ]]; then
  echo "Usage: ${SCRIPT} base_name reference_name variant_name [bug_number]"
  echo "e.g., ${SCRIPT} hatch hatch kohaku b:140261109"
  echo "e.g., ${SCRIPT} zork trembyle dalboz"
  echo "* Adds a new variant of the baseboard to Kconfig and Kconfig.name"
  echo "* Copies the template files for the baseboard to the new variant"
  exit 1
fi

to_lower() {
  # Convert a string to lower case using ASCII translation rules.
  LC_ALL=C LOWER="${1,,}"
  echo "${LOWER}"
}

to_upper() {
  # Convert a string to upper case using ASCII translation rules.
  LC_ALL=C UPPER="${1^^}"
  echo "${UPPER}"
}

# This is the name of the base board
BASE="$(to_lower "$1")"
# This is the name of the reference board that we're using to make the variant.
# TODO(b/149391804): fix the baseboard/reference board/variant terminology
REFERENCE="$(to_lower "$2")"
# This is the name of the variant that is being cloned.
VARIANT="$(to_lower "$3")"
VARIANT_UPPER="$(to_upper "${VARIANT}")"

# Assign BUG= text, or "None" if that parameter wasn't specified.
BUG="${4:-None}"

# Get the directory where this script is located; it is also where
# kconfig.py is located.
pushd "${BASH_SOURCE%/*}"
SRC=$(pwd)
popd

# The template files are in ${CB_SRC_DIR}/util/mainboard/google/${REFERENCE}/template
TEMPLATE="${CB_SRC_DIR}/util/mainboard/google/${REFERENCE}/template"

# We need to create files in ${CB_SRC_DIR}/src/mainboard/google/${BASE}/variants/${VARIANT}
if [[ ! -e "${CB_SRC_DIR}/src/mainboard/google/${BASE}" ]]; then
  echo "The baseboard directory for ${BASE} does not exist."
  exit 1
fi
pushd "${CB_SRC_DIR}/src/mainboard/google/${BASE}"

# Make sure the variant doesn't already exist.
if [[ -e "variants/${VARIANT}" ]]; then
  echo "variants/${VARIANT} already exists."
  echo "Have you already created this variant?"
  exit 1
fi

# Start a branch. Use YMD timestamp to avoid collisions.
DATE="$(date +%Y%m%d)"
git checkout -b "coreboot_${VARIANT}_${DATE}"

# TODO(b/149701259): trap a function at exit that cleans this up
# Copy the template tree to the target.
mkdir -p "variants/${VARIANT}/"
cp -pr "${TEMPLATE}/." "variants/${VARIANT}/"
if [[ -e "variants/${VARIANT}/Kconfig" ]]; then
  sed -i -e "s/BOARD_GOOGLE_TEMPLATE/BOARD_GOOGLE_${VARIANT_UPPER}/" \
    "variants/${VARIANT}/Kconfig"
fi
git add "variants/${VARIANT}/"

# Now add the new variant to Kconfig and Kconfig.name
# These files are in the current directory, e.g. src/mainboard/google/hatch
"${SRC}/kconfig.py" --board "${REFERENCE}" --variant "${VARIANT}"

mv Kconfig.new Kconfig
mv Kconfig.name.new Kconfig.name

git add Kconfig Kconfig.name

# Now commit the files. Use fmt to word-wrap the main commit message.
MSG=$(echo "Create the ${VARIANT} variant of the ${REFERENCE} reference
board by copying the template files to a new directory named for the
variant." | fmt -w 70)

git commit -sm "${BASE}: Create ${VARIANT} variant

${MSG}

(Auto-Generated by ${SCRIPT} version ${VERSION}).

BUG=${BUG}
BRANCH=None
TEST=util/abuild/abuild -p none -t google/${BASE} -x -a
make sure the build includes GOOGLE_${VARIANT_UPPER}"
# TODO(b/149702214): verify that it builds correctly
