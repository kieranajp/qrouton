---
name: qrouton-slides
description: Author a deck-shaped Markdown artifact that qrouton renders as slides. Use when asked to write a presentation, deck, or talk for a qrouton session. Do not use to read or summarise a deck that already exists.
---

# Write a deck

A deck is one Markdown file under `thoughts/`, opened with `open_file` like any other artifact. What
makes it slides is its frontmatter:

```markdown
---
marp: true
---
```

Nothing else selects the pane. A deck keeps whatever artifact kind its path gives it, so a spec can
be deck-shaped and stay a spec.

Slides are separated by `---` alone on a line, with a blank line before it. The pane draws each one
as a 16:9 card, in qrouton's own colours, with its speaker notes as prose beneath.

## The vocabulary is closed

You get **four layouts** and **five components**, and standard Markdown. There is nothing else.

| Layout | For | You write |
| --- | --- | --- |
| content | The default: heading and body. | nothing |
| title | Deck or section opener. | `<!-- _class: title -->` |
| statement | One sentence, full bleed. The punchline slide. | `<!-- _class: statement -->` |
| alt | Inverted ground, to break rhythm between runs of content. | `<!-- _class: alt -->` |

| Component | For | You write |
| --- | --- | --- |
| cols | Two side-by-side columns. | `<div class="cols">` with two child `<div>`s; add `wide-left` or `wide-right` to unbalance |
| shot | A framed image or video with optional caption. | `<figure class="shot">` holding an `<img>` or `<video>` and an optional `<figcaption>` |
| cards | A row of equal-width cards. | `<div class="cards">` with child `<div>`s; `accent` promotes one |
| callout | A bordered aside. | `<div class="callout note\|good\|warn">` |
| note | A small muted aside, no box. | `<p class="note">` |

Worked examples of every one are in `references/vocabulary.md` beside this file. Read it before
writing a deck that uses anything past a heading and a list.

## Where the line sits

An unrecognised class renders as an unstyled block — visibly wrong, not silently half-right. So:

- **No new class names.** A shape that recurs across decks earns a name in the theme first, which is
  a change to qrouton, not to your deck.
- **No bespoke layouts, and no per-deck `<style>` block.** Both are dropped or ignored.
- **No inline `style` attribute.** Marp's HTML allowlist strips it; `cols wide-left` is what replaces
  the one legitimate use.
- **No `theme:` directive.** There is one theme and it is the app's.
- **No background-image directives**, and no colour, font or size of your own. Every value in the
  theme is a token the app serves, which is why a deck matches the workbench it opens in.

"Can I have a custom layout for this one slide" is a documented no.

## Notes and media

A plain HTML comment is a speaker note, rendered under its card:

```markdown
<!-- Why this slide is here, in a sentence or two. -->
```

An underscore-prefixed comment is a directive to Marp and is not shown: `<!-- _class: title -->`,
`<!-- _paginate: false -->`.

Images and video are relative to the deck: `<img src="./shot.png">`, `<video src="./clip.mp4">`,
`![alt](./shot.png)`. The workbench serves them from the deck's own directory, so keep them beside
it and inside the session. Only pictures and video resolve; anything else is a 404.

## Before you hand it over

- One idea per slide. A slide that needs a scrollbar is two slides.
- Write the note. In a reading pane there is no second screen, so the note is the argument the slide
  is too short to carry.
- Open it with `open_file` and look at it. An unstyled block means you invented a class name.
