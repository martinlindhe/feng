# feng - a very data driven file format template system

STATUS: private draft work

# Features
- universal binary format template language (WIP)
- command line print-out of binary file structure (WIP)
- TODO: auto validate file checksums
- TODO: extract data from file







# TODO 5 mars 2021:

CURRENT TODO 1. finish rewrite (data to layout mapping, final layout parsing, present listing)


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




- low prio TODO: terminal ui hex editor, like in formats cmd. proof of concept and dogfooding



# SIMILAR WORKS


http://www.nyangau.org/be/be.htm - TODO evaluate

Hexinator / Synalyze It! - Universal Parsing Engine . TODO evaluate
    "Hexinator is freemium version of Synalyze It!"
    https://github.com/synalysis/Grammars/blob/master/bitmap.grammar


quickbms - xxx

010 editor templates
    https://www.sweetscape.com/010editor/repository/templates/

hex fiend templates




FIXME: be scoop install for http://www.nyangau.org/be/be.zip    ???
        cant work: be.zip is bad url, no versioning and can be replaced at any time, hash validation wont work
    or just use hash for the known version in scoop formula!!!
