# feng - a very data driven file format template system

Proof of concept reference implementation is being written in golang for
quick iteration, while a rust or zig (explore integration possibilities) implementation is planned.

STATUS: private draft work

# Features
- universal binary format template language (WIP)
- command line print-out of binary file structure (WIP)
- TODO: auto validate file checksums
- TODO: extract data from file







# TODO 5 mars 2021:

CURRENT TODO 1. finish rewrite (data to layout mapping, final layout parsing, present listing)


TODO soon ? : a lot of data need "" escape in yaml, so might as well go to toml before more templates will need to convert?
    - pro/con




TODO 1000. example client app in a blog about it (need easy to use end-user API)
    show how to make a gameboy rom header frontend using template or something






# FUTURE POSSIBLE FEATURES

- render the yaml template representation as a zim language input data validator (code-gen)

TODO 3. cli - checksum field validator


- universal raw data expander:
    - untyped data (raw)
    - RGBA and other image formats
    - combine RGB + palette sections into something and expand it to uncompressed image data
    - expand zip archive file and preserve filename, file attributes and uncompressed content
    - pack data into a format from input data. most compression archives needs a starting file/folder (some only a file), image needs raw image data, audio needs raw audio data.
        this would make it possible to transcode between two formats described as a yml template


- grammar-level diffs
    proof of concept should just print-out data and do text diff


- LATER: universal unpackers
    scriptable or template(??): describe unpacker for old-school MS-DOS packers

- LATER: processing templates: a template language to describe how to manipulate data stream,
    to implement custom manipulation like rot13

- LATER: importer helper from https://github.com/synalysis/Grammars format ?


- LATER: "gnu file" lookalike ?



- low prio TODO: terminal ui hex editor, like in formats cmd. proof of concept and dogfooding


# SCRIPTING

later: maybe https://mun-lang.org/ ?
    has the needed native integer types



# SIMILAR WORKS

kaitai
    https://github.com/kaitai-io/kaitai_struct
    https://github.com/kaitai-io/awesome-kaitai
    http://formats.kaitai.io/dos_datetime/index.html

    similar in spirit but different end goals - TODO evaluate proper!!!
    - they compile to source from declaration as the end goal (?)
    - we want to make available the result in many kinds of tooling as an end goal
    - we want to make available C library for other hex editors to use the result, in order to focus only on the formats.
    - we handle special types of data such as checksums and image data, they just treat them as unknown bytes
    - their format is over-complicated and hard to read


Hexinator / Synalyze It! - Universal Parsing Engine
    TODO evaluate
    "Hexinator is freemium version of Synalyze It!"
    https://github.com/synalysis/Grammars/blob/master/bitmap.grammar




quickbms - xxx


010 editor templates
    https://www.sweetscape.com/010editor/repository/templates/


hex fiend templates
    https://github.com/HexFiend/HexFiend/tree/master/templates


http://www.nyangau.org/be/be.htm - TODO evaluate
