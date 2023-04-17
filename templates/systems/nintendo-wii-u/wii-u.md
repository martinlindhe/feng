For decompilation of Wii-U apps, they first need to be decrypted.


# software
Switch-Toolbox viewer & extractor for various Wii-U/Switch formats.
https://github.com/KillzXGaming/Switch-Toolbox


# Overview of Wii-U file formats

https://www.retroreversing.com/WiiUFileFormats


# cdecrypt
cdecrypt can decrypt a folder with app/h3/cert/tik/tmd files (Nintendo Update Server files).

https://github.com/VitaSmith/cdecrypt

    paru -S cdecrypt-git

    cdecrypt folder



# wux to wud
WUX is a compressed format for Wii-U games (WUD)

    paru -S wudcompress

    wudcompress file.wux


# extract wud files

    https://github.com/martinlindhe/wud (fork that takes keys as hex strings, feb 2023)

    wud extract game.wud commonKey gameKey

