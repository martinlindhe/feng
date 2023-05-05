software:
  - https://github.com/jakcron/nstool
  - https://github.com/SciresM/hactool
  - https://github.com/Thealexbarney/LibHac
  - dump switch games to nsp https://github.com/DarkMatterCore/nxdumptool


# decompress nsz to nsp

$ nsz -D file.nsz -o .


# extract nsp, nsz, xci
```
$ nsz -x file.{nsp,nsz,xci} -o out           # pacman -S nsz

$ hactool -t pfs0 --outdir=out file.nsp      # paru -S hactool-git

$ nstool -x out file.{nsp,xci,nca}
```


# extract romfs
```
$ hactool --romfsdir file-romfs file.nca
```


# extract szs

xxx

# decrypt nca

$ nstool -x dec --tik file.tik file.nca

$ hactool -t nca file.nca