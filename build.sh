#!/bin/bash -e 
 
# for os in darwin windows linux
for os in windows linux
do
	fyne-cross ${os} --pull --arch=*
done

# fyne-cross --pull android
#
# # Hack to deal with a bug in the android cross 
# # compile container.
# if ! grep -q 'LinuxAndBSD' FyneApp.toml ; then
#	cat linux-block.toml >> FyneApp.toml
# fi

# fyne package --target web

mkdir -p fyne-cross/packages || true

for dir in fyne-cross/dist/windows-*
do
	cp ${dir}/KarmaManager.exe.zip fyne-cross/packages/KarmaManager_$(basename ${dir} | sed -e 's/-/_/').exe.zip
done

for dir in fyne-cross/dist/linux-*
do
	cp ${dir}/KarmaManager.tar.xz fyne-cross/packages/KarmaManager_$(basename ${dir} | sed -e 's/-/_/').tar.xz
done

fyne package --target darwin

zipfile="KarmaManager_darwin-arm64.zip"
zip -r ${zipfile} KarmaManager.app
mv ${zipfile} fyne-cross/packages/.

