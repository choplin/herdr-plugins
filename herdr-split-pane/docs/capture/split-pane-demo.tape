Require herdr
Require lazygit

Output "../split-pane-demo.gif"

Set Shell bash
Set FontFamily "HackGen Console NF"
Set FontSize 20
Set Width 1868
Set Height 1068
Set Theme "Catppuccin Mocha"
Set Padding 20
Set TypingSpeed 35ms
Set CursorBlink false

Hide
Type "/tmp/herdr-split-pane-showcase-config/bin/launch-herdr"
Enter
Wait+Screen@15s /❯/
Show
Sleep 800ms
Type@25ms "# Ctrl+G opens lazygit in a split pane."
Enter
Sleep 300ms
Type@25ms "# The pane closes when lazygit exits."
Enter
Sleep 600ms
Hide
Type "open-in-split lazygit right"
Enter
Wait+Screen@15s /Unstaged/
Show
Sleep 2200ms
Type "q"
Sleep 1s
