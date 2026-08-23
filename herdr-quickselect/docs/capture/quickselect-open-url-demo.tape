Require herdr

Output "../../assets/quickselect-open-url.gif"

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
Type@25ms "# Ctrl+Shift+P opens a visible URL."
Enter
Sleep 300ms
Type "cat release.txt"
Enter
Wait+Screen /Docs      https:\/\/herdr\.dev\/plugins\/quick-select/
Sleep 700ms
Hide
Ctrl+P
Wait+Screen@15s /attps/
Show
Sleep 1600ms
Hide
Type "a"
Wait+Screen@15s /Opened URL in browser/
Show
Sleep 2500ms
