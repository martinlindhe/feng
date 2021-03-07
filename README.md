# feng - a very data driven file format template system

STATUS: private draft work

# 5 mars 2021:

CURRENT TODO 1. finish rewrite (data to layout mapping, final layout parsing, present listing)

TODO 2. cli - file structure listing, meaning + actual data

TODO 3. cli - checksum field validator

TODO 5. cli - terminal ui hex editor, like in formats cmd

TODO 999. FINAL GOAL: expander - extract all data objects from file, such as:
    - untyped data (raw)
    - RGBA and other image formats
    - combine RGB + palette sections into something and expand it to uncompressed image data
    - expand zip archive file and preserve filename, file attributes and uncompressed content
    - pack data into a format from input data. most compression archives needs a starting file/folder (some only a file), image needs raw image data, audio needs raw audio data.
        this would make it possible to transcode between two formats described as a yml template

TODO 1000. example client app in a blog about it (need easy to use end-user API)
    show how to make a gameboy rom header frontend using template
