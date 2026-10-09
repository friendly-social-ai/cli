# Custom keys

Put a keymap at `~/.config/friendly/keys.toml`, or at `$XDG_CONFIG_HOME/friendly/keys.toml` when `XDG_CONFIG_HOME` is
set. `FRIENDLY_CONFIG_DIR` picks another folder, see [config.md](config.md#another-folder). List only the actions you
change. Everything else keeps its default. [config/keys.toml](../config/keys.toml) lists every action with its default
keys, ready to copy.

```toml
[navigation]
down  = ["j", "ctrl+j"]   # a list gives several keys, the first shows in hints
last  = "g t"             # a sequence, g then t

[community]
reply  = "m"
delete = []               # unbinds the action
```

## Rules

- A key is one character, or a named key with optional modifiers, like `ctrl+d`, `alt+enter` or `shift+tab`.
- Modifiers go in this order: `ctrl`, `alt`, `shift`, `meta`, `hyper`, `super`.
- Named keys are `enter`, `tab`, `backspace`, `esc`, `space`, `up`, `down`, `left`, `right`, `home`, `end`, `pgup`,
  `pgdown`, `insert`, `delete`, `begin`, `find`, `select` and `f1` to `f99`.
- Write a shifted letter as the capital, `J` and not `shift+j`.
- A key with spaces is a sequence, pressed one key after another, like `g t`. Help shows it as `gt`.
- While a sequence waits for its next key, the footer lists the keys that complete it. Any other key cancels it.
- There is no timeout, so a key can't also start a longer key on the same screen. `g` and `g t` together fail, while
  `g g` and `g t` share their start and work.
- Sequences don't work while you type, in the composer and in forms.
- `navigation` and `common` actions need at least one key. Other actions can be unbound with `[]`.
- Keys the composer and forms use while you type must not be text, so pick one with `ctrl` or `alt`.
- `ctrl+c` always quits.

A file with mistakes stops the app before it starts and lists every mistake. Examples are a key used twice on one
screen, or a screen key that a navigation key would take first. Nothing from the file applies until you fix it.

## Actions

| Section | Action | Default |
|---|---|---|
| `navigation` | `quit` | `q` |
| | `help` | `?` |
| | `open` | `l`, `enter`, `right` |
| | `back` | `h`, `left` |
| | `down` | `j`, `down` |
| | `up` | `k`, `up` |
| | `first` | `g g` |
| | `last` | `G` |
| | `half_page_down` | `ctrl+d` |
| | `half_page_up` | `ctrl+u` |
| | `tab_1` to `tab_4` | `1` to `4` |
| `common` | `cancel` | `esc` |
| | `confirm` | `enter` |
| | `next_field` | `tab`, `down` |
| | `previous_field` | `shift+tab`, `up` |
| | `complete` | `tab` |
| | `next_suggestion` | `down`, `ctrl+n` |
| | `previous_suggestion` | `up`, `ctrl+p` |
| | `refresh` | `R` |
| | `filter` | `/` |
| `community` | `new_post` | `n` |
| | `reply` | `r` |
| | `links` | `o` |
| | `copy` | `y` |
| | `author` | `@` |
| | `edit` | `e` |
| | `delete` | `d` |
| | `next_reply` | `J` |
| | `previous_reply` | `K` |
| | `root` | `H` |
| | `menu` | `ctrl+o` |
| | `post` | `alt+enter` |
| | `editor` | `e` |
| | `preview` | `p` |
| | `attach` | `a` |
| | `paste_image` | `v` |
| | `discard` | `x` |
| | `indent` | `tab` |
| | `outdent` | `shift+tab` |
| `activity` | `next_unread` | `n` |
| | `read_all` | `m` |
| `people` | `connect` | `a` |
| | `skip` | `x` |
| `profile` | `edit` | `e` |
| | `toggle_email` | `v` |
| | `logout` | `x` |
| `user` | `connect` | `a` |
| | `remove` | `x` |
| | `open_social` | `o` |
