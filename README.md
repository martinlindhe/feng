# feng - a very data driven file format template system

[![Discord](https://img.shields.io/discord/999601338407190569.svg?label=&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2)](https://discord.gg/mYBn9XqRBr)

The reference implementation is being written in golang for quick iteration, while a rust
or zig (explore integration possibilities) implementation is planned.


# Feature overview
- universal binary format template language
- command line print-out of binary file structure
- extract raw and compressed data from file (zlib, deflate, lz4)


# Template example

Binary formats are specified in a template file, see [TEMPLATE.md](TEMPLATE.md) for the format documentation.

Here is an example demonstrating the OpenType Font parsing:

```yaml
endian: big

magic:
  - offset: 0000
    match: c'OTTO'

structs:
  header:
    u32 Magic: c'OTTO'
    u16 TableCount: ??
    u16 SearchRange: ??
    u16 EntrySelector: ??
    u16 RangeShift: ??

  table:
    ascii[4] Tag: ??
    label: self.Tag

    u32 Checksum: ??
    u32 Offset: ??
    u32 Length: ??

    offset: self.Offset
    u8[self.Length] Data: ??
    u8[alignment(self.Length, 4)] Padding: ??
    offset: restore

layout:
  - header Header
  - table[Header.TableCount] Table
```
*Taken from the [templates/fonts/otf.yml](templates/fonts/otf.yml) template.*



# Installation

The software is still in alpha state, so no releases have yet been made.

Assuming you have golang, then installing the commandline is as easy as:

    go install github.com/martinlindhe/feng/cmd/feng@latest



# Command line tools

```
cmd/feng: Lists data fields of input file.
cmd/renamer: Renames all files in input folder to use the correct file extensions.
```





# FUTURE POSSIBLE FEATURES

- render the yaml template representation as a zim language input data validator (code-gen)


- universal raw data expander:
* untyped data (raw)
* RGBA and other image formats
* combine RGB + palette sections into something and expand it to uncompressed image data
* expand zip archive file and preserve filename, file attributes and uncompressed content
* pack data into a format from input data. most compression archives needs a starting file/folder (some only a file), image needs raw image data, audio needs raw audio data.
    this would make it possible to transcode between two formats described as a yml template


- universal unpackers
    scriptable or template(??): describe unpacker for old-school MS-DOS packers

- processing templates: a template language to describe how to manipulate data stream,
    to implement custom manipulation like rot13

- importer helper from https://github.com/synalysis/Grammars format ?

- "gnu file" lookalike ?

- "binwalk" lookalike ?

- low prio: terminal ui hex editor, like in https://github.com/martinlindhe/formats/tree/master/cmd/formats (proof of concept and dogfooding)




# SIMILAR WORKS

### kaitai
- https://github.com/kaitai-io/kaitai_struct
- https://github.com/kaitai-io/awesome-kaitai
- http://formats.kaitai.io/dos_datetime/index.html

similar in spirit but different end goals(?)
- they compile to source from declaration as the end goal (?)
- we want to make available the result in many kinds of tooling as an end goal
- we want to make available C library for other hex editors to use the result, in order to focus only on the formats.
- we handle special types of data such as checksums and image data, they just treat them as unknown bytes
- their format is over-complicated and hard to read


### Hexinator / Synalyze It! - Universal Parsing Engine
- "Hexinator is freemium version of Synalyze It!"
- https://github.com/synalysis/Grammars/blob/master/bitmap.grammar

### quickbms
- http://aluigi.altervista.org/quickbms.htm

### "Game Extractor" by "WATTO"
 - http://www.watto.org/game_extractor.html


### 010 editor templates
- https://www.sweetscape.com/010editor/repository/templates/


### hex fiend templates
- https://github.com/HexFiend/HexFiend/tree/master/templates


### malcat - has some form of binary templates
- https://malcat.fr/


### Andys Binary Folding Editor
- http://www.nyangau.org/be/be.htm


### winhex templates
- https://www.x-ways.net/winhex/templates/index.html


### Noesis
- Noesis is a tool for previewing and converting between hundreds of model, image, and animation formats.
- http://richwhitehouse.com/index.php?content=inc_projects.php&showproject=91
- https://github.com/RoadTrain/noesis-plugins
- https://github.com/RoadTrain/noesis-plugins-official


### Ninja ripper
- extract individual models from games
- https://ninjaripper.com/



### TODO evaluate rust relatives mentioned here
- "rkyv is faster than {bincode, capnp, cbor, flatbuffers, postcard, prost, serde_json}"
- STATUS: we could build generators for bincode or whatever from our source format, in addition to pure no dependencies-rust
- https://davidkoloski.me/blog/rkyv-is-faster-than/




# RESOURCES
- http://fileformats.archiveteam.org/wiki/File_Formats
- https://www.fileformat.info/
- https://github.com/kaitai-io/kaitai_struct_formats
- https://wiki.multimedia.cx/index.php/Main_Page
- http://wiki.xentax.com/index.php/Category:File_Format




# GAME MODDING RESOURCES
- https://moddingwiki.shikadi.net/wiki/Main_Page     (DOS Game Modding Wiki)
- http://wiki.xentax.com/index.php/Game_File_Format_Central
- https://zenhax.com/
- https://www.vg-resource.com/
- https://www.gildor.org/smf/index.php/board,3.0.html - UE viewer forums (unreal engine extractor)
- https://www.hackromtools.info/ - Pokemon modding
- https://psxtools.de/forum/


# SAMPLES
- https://telparia.com/fileFormatSamples/
