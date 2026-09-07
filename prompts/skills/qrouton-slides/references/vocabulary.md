# The nine names, worked

One example per layout and per component. Everything here renders; anything not here does not.

## The file

```markdown
---
marp: true
---

<!-- _class: title -->

# The deck's title

## The line under it

<!-- The opener. A note is a plain HTML comment and renders beneath the card. -->

---

## The first real slide

Body copy.
```

`marp: true` is the whole of the frontmatter a deck needs. `theme:`, `size:` and `paginate:` are
ignored or already set; leave them out.

A slide break is `---` alone on a line with a blank line above it. Inside a fenced code block it is
just text, and directly under a line of prose it is a Markdown heading underline, not a break.

## Layouts

A layout is chosen with an underscore-prefixed comment, which applies to its own slide only.

### content

The default. Write nothing.

```markdown
## What changed

- The runner now resumes its own conversation
- `--resume` is no longer passed by the launcher

A closing sentence.
```

Headings, lists, tables, blockquotes, links, `code` spans and fenced code blocks all render. A table
is the right shape for a paired list; there is no component for one.

### title

```markdown
<!-- _class: title -->

# Seven weeks, 377 commits

## What came out of it
```

Large centred title with a machine-face line under it. Use it for the deck's opener and for section
breaks inside a long deck.

### statement

```markdown
<!-- _class: statement -->

# Oops.

Seven weeks. 377 commits. 54,000 lines.
```

One sentence, full bleed, in the label accent. The paragraph under it is optional and quieter. If
you find yourself writing three of these in a row, they are content slides.

### alt

```markdown
<!-- _class: alt -->

## Last time, briefly

- The recap nobody needs but everybody wants
```

An inverted ground and nothing else. Its job is rhythm: one after a run of four or five content
slides, not every other slide.

## Components

Components compose inside any layout.

### cols

```markdown
<div class="cols wide-left"><div>

The argument, which needs the room.

</div><div>

The aside, which does not.

</div></div>
```

Exactly two children. `cols` alone splits them evenly; `wide-left` and `wide-right` give the named
side roughly twice the width. Leave a blank line inside each `<div>` so Markdown inside it is still
parsed as Markdown.

### shot

```markdown
<figure class="shot">
  <img src="./claude-design.png">
  <figcaption>The picker, mid-assembly</figcaption>
</figure>
```

Video is the same shape:

```markdown
<figure class="shot">
  <video src="./iter-tui.mp4" autoplay loop muted playsinline></video>
</figure>
```

The path is relative to the deck. The caption is optional and sets in the instruction face.

### cards

```markdown
<div class="cards">
<div>Research</div>
<div class="accent">Plan</div>
<div>Implement</div>
</div>
```

A row of equal-width cards. `accent` promotes exactly one — if two are important, none are.

### callout

```markdown
<div class="callout note">The neutral aside.</div>

<div class="callout good">The thing that survived.</div>

<div class="callout warn">The thing to watch.</div>
```

`note`, `good` and `warn` change the left edge only. A bare `<div class="callout">` is the same box
in the default edge.

### note

```markdown
<p class="note">Verdict at the time: very cool, and not for us.</p>
```

A small muted aside with no box. Use it for the line that qualifies a slide without belonging in its
body.

## Things that will not work

| You write | What happens |
| --- | --- |
| `<div class="ledger">` | An unstyled block, in body type on the slide's ground |
| `style="flex: 1.9"` | Dropped by the HTML allowlist; use `cols wide-left` |
| `<style>` in the deck | Ignored |
| `theme: something` | Ignored; there is one theme |
| `<img src="https://…">` | Undefined — keep media beside the deck |
| `aria-label` on anything | Dropped by the allowlist |
