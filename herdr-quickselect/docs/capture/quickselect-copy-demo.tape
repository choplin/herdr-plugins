Require herdr
Require pbpaste

Output "../../assets/quickselect-copy.gif"

Set Shell bash
Set FontFamily "HackGen Console NF"
Set FontSize 20
Set Width 1868
Set Height 800
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 35ms
Set CursorBlink false

Hide
Type "/tmp/herdr-quickselect-showcase-config/bin/launch-herdr"
Enter
Wait+Screen@15s /❯/
Show
Sleep 800ms
Type@25ms "# Ctrl+Shift+X copies a visible terminal value."
Enter
Sleep 300ms
Type "cat release.txt"
Enter
Wait+Screen /Artifact  \/opt\/herdr\/plugins\/quickselect-v0\.3\.0\.tar\.gz/
Sleep 700ms
Hide
Ctrl+X
Wait+Screen@15s /aopt/
Show
Sleep 1600ms
Hide
Type "a"
Wait+Screen@15s /Copied to clipboard/
Show
Sleep 1800ms
Type@25ms "# Paste to verify the selected value."
Enter
Type "pbpaste; echo"
Enter
Wait+Screen /\/opt\/herdr\/plugins\/quickselect-v0\.3\.0\.tar\.gz[[:space:]│]+❯/
Sleep 1800ms
