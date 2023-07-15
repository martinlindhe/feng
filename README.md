[![Discord](https://img.shields.io/discord/999601338407190569.svg?label=&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2)](https://discord.gg/mYBn9XqRBr)

Feng is a command line app for querying, presenting and extracting any kind of structured binary data,
using a yaml-based template format that allows for specifying the file structure.


# Usage

```
Usage: feng <filename>

A binary template reader and data presenter.

Arguments:
  <filename>

Flags:
  -h, --help                  Show context-sensitive help.
      --template=STRING       Parse file using this template.
  -x, --extract               Extract data streams from input file.
      --out-dir=STRING        Write files to this directory. Implies --extract
      --offset=INT-64         Starting offset (default is 0).
      --raw                   Show raw values
      --local-time            Show timestamps in local timezone (default is UTC).
      --brief                 Show brief file information.
      --tree                  Show parsed file structure tree.
      --decimal               Show offsets in decimal (default is hex).
      --unmapped              Print a report on unmapped bytes.
      --overlapping           Print a report on overlapping bytes.
      --debug                 [Dev] Enable debug logging
      --time                  [Dev] Measure where processing time is spent.
      --cpu-profile=STRING    [Dev] Create CPU profile.
      --mem-profile=STRING    [Dev] Create memory profile.
```


# Installation

Binary releases can be downloaded from the [Releases](https://github.com/martinlindhe/feng/releases) page.


## Installation from git

You need `golang` installed on your system. Install the `feng` command:

    go install github.com/martinlindhe/feng/cmd/feng@latest


# Template example

Binary formats are specified in a template file. See [TEMPLATE.md](TEMPLATE.md) for the template format documentation.

Here's the OpenType Font table parsing:

```yaml
endian: big

magic:
  - offset: 0000
    match: c'OTTO'

structs:
  header:
    ascii[4] Magic: c'OTTO'
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

Mapping a file with the above structure would result in something like [smoketest/reference/fonts/otf](smoketest/reference/fonts/otf).

*Template is from [templates/fonts/otf.yml](templates/fonts/otf.yml).*



## User templates

feng reads user templates from `~/.config/feng`, or from a custom path with the `--template` argument.
