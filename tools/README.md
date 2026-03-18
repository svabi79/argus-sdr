# Decoder Tools

Place external decoder binaries here and update `config.yaml` decoder commands.

Examples (Windows):
- `tools/ft8/ft8_decoder.bat`
- `tools/wspr/wspr_decoder.bat`
- `tools/dmr/dmr_decoder.bat`
- `tools/dstar/dstar_decoder.bat`
- `tools/fsk/fsk_decoder.bat`
- `tools/psk/psk_decoder.bat`

Each script should accept either IQ or audio:
```
--iq <path> --sample-rate <sr>
```
Or:
```
--audio <path> --sample-rate <sr>
```

The app replaces `{iq}`, `{sr}`, and `{audio}` placeholders in config commands.

Downloaded:
- WSJT-X installer: tools/wsjtx/wsjtx-2.7.0-win64.exe
- fldigi installer: tools/fldigi/fldigi-latest.exe
- dsd-neo binary: tools/dsd-neo/bin/dsd-neo.exe
