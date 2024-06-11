# MP4META
MP4Meta is a high level implementation of abema's go-mp4, licensed under the MIT license, link in Acknowledgements. 
This library is both an adapter and facade implementation for abema's low level I/O interfaces. By utilizing this library,
we are able to interact with almost all relevent m4a /m4b metadata tags. At the same time, we are able to keep mdat in sync
when editing the metadata.

## Features
- Read and write MP4 atoms (m4a, m4b): "artist", "albumArtist", "album", "coverArt", "comments", "composer", "copyright", "genre", 
"title", "year", "encoder"
- Reads and writes "trackNumber", "trackTotal", "discNumber", "discTotal" and "tempo (bpm)" tags with ease
- Everything's built in, plug and play, with a simple interface, compatible with [AudioMeta v3]("github.com/gcottom/audiometa/v3"),
for more audio formats. 

## Acknowledgements
- [go-mp4](github.com/abema/go-mp4): Abema's go-mp4 makes this library possible. They provided the low level and I made it high level,
for m4a, and m4b. With this combination, we can write meta tags with ease. 

## License
This project is licensed under the MIT License. See the LICENSE file for details.

Parts of this project include third-party libraries under the MIT license. See the LICENSE file for details.

## Related Links
[AudioMeta v3](github.com/gcottom/audiometa/v3)

[MP3Meta](github.com/gcottom/mp3meta)

[OGGMeta](github.com/gcottom/oggmeta)

[FLACMeta](github.com/gcottom/flacmeta)