#!/bin/bash
set -ueo pipefail
SCRIPT=`realpath $0`
SCRIPT_PATH=`dirname $SCRIPT`

version=$(git describe --tag)

cd $SCRIPT_PATH/..

for linux_arch in 386 amd64
do
    deb_arch=$linux_arch
    [[ "$deb_arch" = "386" ]] && deb_arch=i386
    pkg_name=capitan_${version}_$deb_arch
    env GOOS=linux GOARCH=$linux_arch go build -o build/releases/${pkg_name}/capitan
    (cd build/releases/${pkg_name} && zip ../../${pkg_name}.zip  capitan)
    mkdir -p build/releases/${pkg_name}/usr/local/bin
    mv  build/releases/${pkg_name}/capitan  build/releases/${pkg_name}/usr/local/bin/capitan
    mkdir -p build/releases/${pkg_name}/DEBIAN
    cat << EOF > build/releases/${pkg_name}/DEBIAN/postinst
#!/bin/sh
set -e
echo 'Installed capitan'
EOF
    chmod +x build/releases/${pkg_name}/DEBIAN/postinst

    cat << EOF > build/releases/${pkg_name}/DEBIAN/prerem
#!/bin/sh
set -e
echo 'Removing capitan...'
EOF
    chmod +x build/releases/${pkg_name}/DEBIAN/prerem

    cat << EOF >  build/releases/${pkg_name}/DEBIAN/control
Package: capitan
Version: $version
Section: base
Priority: optional
Architecture: $deb_arch
Maintainer: Donal Byrne <byrnedo@tcd.ie>
Description: Scriptable docker container orchestration
 Scriptable docker container orchestration
EOF
    (cd build/releases && dpkg-deb --build ${pkg_name} && mv ${pkg_name}.deb ../)

done
