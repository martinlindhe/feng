software:
  - https://github.com/jakcron/nstool
  - https://github.com/SciresM/hactool
  - https://github.com/Thealexbarney/LibHac
  - dump switch games to nsp https://github.com/DarkMatterCore/nxdumptool


# extract nsp, xci
```
$ hactool -t pfs0 --outdir=out file.nsp      # paru -S hactool-git

$ nsz -x file.{nsp,nsz,xci} -o out           # pacman -S nsz

$ nstool -x out file.{nsp,xci}
```


# extract romfs
```
$ hactool --romfsdir file-romfs file.nca
```


# extract szs

xxx

# decrypt nca

$ hactool -t nca file.nca

$ nstool -x dec --tik file.tik file.nca

