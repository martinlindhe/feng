# Extract sound from .bank files

RIFF FEV FMT files is extracted with https://github.com/vgmstream/vgmstream

    paru -S vgmstream


Extract all sub-streams to files named `<stream name>.wav`:

    vgmstream-cli file.bank -o "?n.wav" -S 0


# References

- https://github.com/imadr/Unity-game-hacking
- https://www.kodeco.com/36285673-how-to-reverse-engineer-a-unity-game

