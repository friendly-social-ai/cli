# Custom keys

Put a keymap at `$XDG_CONFIG_HOME/friendly/keys.toml`, or `~/.config/friendly/keys.toml` when `XDG_CONFIG_HOME`
isn't set. List only the actions you change. Everything else keeps its default. [keys.toml](keys.toml) lists every
action with its default keys, ready to copy.

```toml
[navigation]
down  = ["j", "ctrl+j"]   # a list gives several keys, the first shows in hints
first = "g"               # one key or a two-key sequence like "g g"

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
- Only `navigation.first` takes a sequence.
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
| | `refresh` | `r` |
| | `filter` | `/` |
| `community` | `new_post` | `n` |
| | `reply` | `n` |
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
