#!/bin/bash -e

for os in windows linux
do
	fyne-cross ${os} --pull --arch=*
done

mkdir -p fyne-cross/packages || true

for dir in fyne-cross/dist/windows-*
do
	cp ${dir}/KarmaManager.exe.zip fyne-cross/packages/KarmaManager_$(basename ${dir} | sed -e 's/-/_/').exe.zip
done

for dir in fyne-cross/dist/linux-*
do
	cp ${dir}/KarmaManager.tar.xz fyne-cross/packages/KarmaManager_$(basename ${dir} | sed -e 's/-/_/').tar.xz
done

# no need for fyne-cross, because it has a bug in newer cross compiles
fyne package --target darwin

zipfile="KarmaManager_darwin-arm64.zip"
zip -r ${zipfile} KarmaManager.app
mv ${zipfile} fyne-cross/packages/.

