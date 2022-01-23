# feng - a very data driven file format template system

Proof of concept reference implementation is being written in golang for
quick iteration, while a rust or zig (explore integration possibilities) implementation is planned.

STATUS: private draft work

# Features
- universal binary format template language (WIP)
- command line print-out of binary file structure (WIP)
- TODO: auto validate file checksums
- TODO: extract data from file
- TODO: binwalk-type of program


# Command line tools

    cmd/renamer: Renames all files in input folder to use the correct file extensions.






# FUTURE POSSIBLE FEATURES

- render the yaml template representation as a zim language input data validator (code-gen)



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


- LATER: "binwalk" like feature ?



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




quickbms - TODO evaluate


 "Game Extractor" by "WATTO" - TODO evaluate



010 editor templates
    https://www.sweetscape.com/010editor/repository/templates/


hex fiend templates
    https://github.com/HexFiend/HexFiend/tree/master/templates


http://www.nyangau.org/be/be.htm - TODO evaluate



https://davidkoloski.me/blog/rkyv-is-faster-than/
    TODO evaluate rust relatives mentioned here:
        "rkyv is faster than {bincode, capnp, cbor, flatbuffers, postcard, prost, serde_json}"
    STATUS: we could build generators for bincode or whatever from our source format, in addition to pure no dependencies-rust




# ADVANCED SIMILAR WORKS - asset extractors

"Ninja ripper" - TODO evaluate
    extract individual models from games




# RESOURCES

http://fileformats.archiveteam.org/wiki/File_Formats
https://www.fileformat.info/
https://github.com/kaitai-io/kaitai_struct_formats
https://wiki.multimedia.cx/index.php/Main_Page


# GAME MODDING RESOURCES
https://moddingwiki.shikadi.net/wiki/Main_Page      - DOS Game Modding Wiki
http://wiki.xentax.com/index.php/Game_File_Format_Central
https://zenhax.com/
https://www.vg-resource.com/

https://www.gildor.org/smf/index.php/board,3.0.html - UE viewer forums (unreal engine extractor)


# SAMPLES

https://telparia.com/fileFormatSamples/
