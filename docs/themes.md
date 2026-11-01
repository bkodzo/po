# Making your own character

A theme is a JSON file. There are two kinds.

- A **reskin** is the quick way. It takes one of the built-in characters (soccer, ghost or cat) as its base and changes its colours, its mood tints and the words it says. The shapes and animations stay the same.
- A **full pack** spells out every picture, so you can draw a character from scratch. It is what `po theme export` writes.

A theme file is only data. It cannot run code.

## A reskin in five minutes

```
po theme new mango --base cat    # writes mango.json, a copy of the cat to edit
# edit mango.json
po theme validate mango.json
po theme install mango.json
po theme set mango
```

Pick the colours and the words you want to change and delete nothing you do not need to. A file as small as this works:

```json
{
  "name": "mango",
  "base": "cat",
  "palette": { "b": [250, 190, 40] },
  "lines": { "calm": ["Mango says hello."] }
}
```

Anything the file does not mention is taken from the base.

### Palette letters

The pictures are drawn with single letters, and the palette says what colour each one is. `.` is always transparent.

| letter | what it is |
|---|---|
| `o` | outline |
| `b` | body |
| `l` | highlight on the body |
| `s` | shading |
| `f` | feet |
| `w` `k` | eye white and eye pupil |
| `m` | mouth |
| `c` | cheeks and nose |
| `h` | the shadow on the ground |
| `g` `z` `y` | steam, sleep Zs, sparkles |
| `e` | the inside of a cat's ear |

### Tints

Each mood can pull the body colour part of the way toward another colour:

```json
"tints": { "frustrated": { "color": [235, 80, 70], "strength": 0.4 } }
```

`strength` runs from 0 (no change) to 1 (fully that colour). Moods you can tint: `frustrated`, `celebrating`, `tired`, `focused`. Strong tints toward a colour opposite the body turn muddy, so keep them gentle.

### Words

- `lines`: what the pet says for each mood (`calm`, `frustrated`, `celebrating`, `tired`, `focused`). One is chosen at random.
- `unsure`: what it says when it cannot tell.
- `milestones`: lines for notable moments (`record`, `relief`, `late_run`, `comeback`, `busy`, `first_push`). A line may contain one `%d`, which becomes the number the moment is about.
- `recaps`: a verdict for `po recap` (`quiet`, `rough`, `steady`, `great`).

A line must fit the speech bubble: 80 characters at most, no line breaks, and only one `%d`.

## A full pack

A full pack has no `base`. It has a `moods` section with all five moods, and each mood has a `palette` and two animations:

- `full`: the creature itself, from 8 to 64 pixels each way. Every frame is `w * h` letters.
- `mini`: a tiny 8 by 4 version for a terminal corner.

An animation is a list of `frames` plus a `sequence` that says which frame to show at each tick. A frame can be one long string, or, much more readable, a list of rows:

```json
"frames": [
  ["..oooo..",
   ".obbbbo.",
   "obkbbkbo",
   "obbmmbbo"]
]
```

You can also add `activities` (`look`, `stretch`, `sleep`, `wake`, `peek`). Ones you leave out fall back to the calm picture. The quickest way to start is `po theme export mine.json`, which writes the current character as a full pack to edit.

## Checking and installing

`po theme validate` reports every problem at once, with exactly where it is, for example `moods.calm.full.frames[3]: 477 letters, want 480`. It also rejects misspelled fields instead of ignoring them.

Installed themes live in `~/Library/Application Support/po/themes/`. They appear in `po theme list` and in the window's Character menu. `po theme remove <name>` takes one out. The built-in names (`soccer`, `ghost`, `cat`) are reserved.
