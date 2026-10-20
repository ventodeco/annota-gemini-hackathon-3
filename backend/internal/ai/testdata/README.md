# Synthetic routing and audio fixtures

`routing.json` uses synthetic model IDs, `.test` hosts and environment variable names. It is not a production configuration or model recommendation.

`silence.mp3` was generated locally without third-party recordings. It contains four 417-byte MPEG-1 Layer III stereo silence frames: the header bytes `ff fb 90 00`, followed by 413 zero bytes, repeated four times. The stream is 128 kbps at 44.1 kHz. macOS `afconvert silence.mp3 /tmp/annota-silence.wav -f WAVE -d LEI16` decoded it into 4,608 stereo PCM frames. It tests binary/hex transport integrity; it does not qualify Japanese speech quality.
