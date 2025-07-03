#!/bin/bash

echo "########################################################################"
echo "######################## ICC has no tests ##############################"
echo "########################################################################"

# Setup
LOCAL_PWD=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Linters
bash "$LOCAL_PWD"/run-lint.sh -s -c || CATCH=1
