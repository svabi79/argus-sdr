# Decoder Tools

Place external decoder binaries here and update `config.yaml` decoder commands.

Examples (Windows):
- `tools/ft8/ft8_decoder.bat`
- `tools/wspr/wspr_decoder.bat`
- `tools/dmr/dmr_decoder.bat`
- `tools/dstar/dstar_decoder.bat`
- `tools/fsk/fsk_decoder.bat`
- `tools/psk/psk_decoder.bat`

Each script should accept:
```
--iq <path> --sample-rate <sr>
```

The app replaces `{iq}` and `{sr}` placeholders in config commands.
