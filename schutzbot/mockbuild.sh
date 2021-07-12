#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Get OS and architecture details.
source tools/set-env-variables.sh

# Mock configuration file to use for building RPMs.
MOCK_CONFIG="${ID}-${VERSION_ID%.*}-$(uname -m)"

if [[ $ID == centos ]]; then
  MOCK_CONFIG="centos-stream-8-$(uname -m)"
fi

# The commit this script operates on.
COMMIT=$(git rev-parse HEAD)

# Bucket in S3 where our artifacts are uploaded
REPO_BUCKET=osbuild-composer-repos

# Public URL for the S3 bucket with our artifacts.
MOCK_REPO_BASE_URL="http://${REPO_BUCKET}.s3-website.us-east-2.amazonaws.com"

# Relative path of the repository – used for constructing both the local and
# remote paths below, so that they're consistent.
REPO_PATH=osbuild-composer/${ID}-${VERSION_ID}/${ARCH}/${COMMIT}

# Directory to hold the RPMs temporarily before we upload them.
REPO_DIR=repo/${REPO_PATH}

# Full URL to the RPM repository after they are uploaded.
REPO_URL=${MOCK_REPO_BASE_URL}/${REPO_PATH}

# Don't rerun the build if it already exists
if curl --silent --fail --head --output /dev/null "${REPO_URL}/repodata/repomd.xml"; then
  greenprint "🎁 Repository already exists. Exiting."
  exit 0
fi

# TODO: update to install correct version once epel-9 is out
# Mock and s3cmd is only available in EPEL for RHEL.
if [[ $ID == rhel || $ID == centos ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

# Install requirements for building RPMs in mock.
greenprint "📦 Installing mock requirements"
sudo dnf -y install createrepo_c mock s3cmd


# Print some data.
greenprint "🧬 Using mock config: ${MOCK_CONFIG}"
greenprint "📦 SHA: ${COMMIT}"
greenprint "📤 RPMS will be uploaded to: ${REPO_URL}"

if [[ "$ID" == rhel ]] && ! sudo subscription-manager status; then
    greenprint "📋 Updating RHEL 8 mock template with the latest nightly repositories"
    # strip everything after line with # repos
    sudo sed -i '/# repos/q' /etc/mock/templates/rhel-8.tpl
    # remove the subscription check
    sudo sed -i "s/config_opts\['redhat_subscription_required'\] = True/config_opts['redhat_subscription_required'] = False/" /etc/mock/templates/rhel-8.tpl
    # reuse redhat.repo
    cat /etc/yum.repos.d/rhel8internal.repo | sudo tee -a /etc/mock/templates/rhel-8.tpl > /dev/null
    # We need triple quotes at the end of the template to mark the end of the repo list.
    echo '"""' | sudo tee -a /etc/mock/templates/rhel-8.tpl
fi

greenprint "🔧 Building source RPM"
git archive --prefix "osbuild-composer-${COMMIT}/" --output "osbuild-composer-${COMMIT}.tar.gz" HEAD
sudo mock -r "$MOCK_CONFIG" --buildsrpm \
  --define "commit ${COMMIT}" \
  --spec ./osbuild-composer.spec \
  --sources "./osbuild-composer-${COMMIT}.tar.gz" \
  --resultdir ./srpm

greenprint "🎁 Building RPMs"
sudo mock -r "$MOCK_CONFIG" \
    --define "commit ${COMMIT}" \
    --with=tests \
    --resultdir "$REPO_DIR" \
    srpm/*.src.rpm

# Change the ownership of all of our repo files from root to our CI user.
sudo chown -R "$USER" "${REPO_DIR%%/*}"

greenprint "🧹 Remove logs from mock build"
rm "${REPO_DIR}"/*.log

# Create a repo of the built RPMs.
greenprint "⛓️ Creating dnf repository"
createrepo_c "${REPO_DIR}"

# Upload repository to S3.
greenprint "☁ Uploading RPMs to S3"
pushd repo
    s3cmd --acl-public put --recursive . s3://${REPO_BUCKET}/
popd
