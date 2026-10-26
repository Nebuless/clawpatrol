# PRX brand assets

- `prx-icon.svg`: ink mark for light surfaces. `prx-icon-light.svg`: light mark for dark surfaces.
- `prx-icon-mono.svg`: single-color `currentColor` mark for template icons and one-color reproduction.
- `prx-lockup.svg` / `prx-lockup-light.svg`: light/dark-surface lockups with outlined glyphs, no font dependency.
- `prx-app-icon.svg`: violet tile with the white mark. PNG exports are 1024 and 512px.
- `apple-touch-icon.png`: 180px. `favicon-32.png` and `favicon-16.png`: browser fallbacks. `favicon.ico`: 16/32/48px.
- `og-image.svg` / `og-image.png`: outlined source and 1200×630 link preview.

The violet dot reads from 32px. At 16px the mark reads as a framed dot cluster. Keep clear space of at least half the mark height. Do not stretch, add effects, or recolor the lit dot outside the approved variants. Mono is the one-color exception.

Colors: ink `#1b1930`, light ink `#f2f1f9`, violet `#6538d8`, dark-surface lit dot `#bea8ff`, tile lit dot `#d9cdff`, tile marks `#ffffff`, canvas `#f8f8fc`.

Fonts in `shared/fonts/`: Hanken Grotesk 650 for PRX, IBM Plex Mono 500 for NEBULESS, IBM Plex Sans for body text. Lockups match `shared/brand.css`: 32px mark, 25.6px name with -0.02em tracking, 10.88px attribution with 0.14em tracking, 9.6px gaps and 1px divider. Plex Mono 500 outlines interpolate the bundled 400/600 glyphs at their midpoint.

Legacy: `claw-patrol-icon.svg` and `claw-patrol-logo.svg` remain only for compatibility with cached login pages. Do not use them for new product surfaces.
