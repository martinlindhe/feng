# Extract data from ISO

If mounted in ISO9660 mode, it will only expose videos explaining it is a game disc.

Extract the Xbox filesystem using `extract-xiso`

https://github.com/XboxDev/extract-xiso  # paru -S extract-xiso



# Extract XBox LIVE package

https://github.com/Gualdimar/Velocity

http://gael360.free.fr/wxPirs.php


# More file format documentation for Xbox 360

https://free60.org/System-Software/Formats/STFS


# LZX compression (TODO)

Some formats uses Microsoft LZX compression, aka XMemDecompress, aka xcompress.lib

- can maybe repurpose https://github.com/microsoft/go-winio/blob/e268c11e27607f25b97bcb14e9d01af70c2c0f52/wim/decompress.go ?

- rust decompressor https://crates.io/crates/lzxd

- TODO no compressor code ?

https://en.wikipedia.org/wiki/LZX
