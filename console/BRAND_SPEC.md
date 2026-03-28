# Velarix Brand Spec

This file is the canonical visual and editorial source of truth for every Velarix web surface built in `console/`.

## 1. Brand Intent

Velarix presents itself as a high-trust technical operator.

"Cencori-inspired" in this context means:

- severe, restrained, industrial
- dark-first and information-dense
- premium without luxury theatrics
- technical without hacker aesthetics
- calm under pressure

The interface should feel like a control plane, not a marketing toy.

## 2. Non-Negotiables

- Strict dark mode only. No light theme variants.
- Every meaningful boundary uses a visible 1px border.
- Surfaces are flat and precise. Glassmorphism is not allowed.
- Accent color usage is sparse and intentional.
- Motion must clarify state, depth, or causality. Never animate for decoration alone.
- Layout must snap to a 4px spacing grid.

## 3. Color System

### Core Palette

| Token | Hex | Usage |
| --- | --- | --- |
| `background` | `#070A0E` | Global page background |
| `surface` | `#0F141B` | Default cards, panels, navigation, command surfaces |
| `surface-elevated` | `#141B24` | Modals, flyouts, highlighted containers |
| `surface-muted` | `#0B1016` | Inset wells, code blocks, secondary rails |
| `border` | `#242C36` | Standard 1px edge |
| `border-subtle` | `#1A212B` | Nested separators only |
| `text-primary` | `#E7ECF2` | Main copy |
| `text-secondary` | `#A5B0BD` | Supporting copy |
| `text-muted` | `#728090` | Metadata, timestamps, disabled labels |
| `primary-accent-chrome` | `#C7D0DB` | Chrome highlight, premium emphasis, primary outline moments |
| `primary-accent-chrome-pressed` | `#AEB8C5` | Pressed state for chrome-accent controls |

### Logic Blue

Logic Blue is the only saturated brand color family. It signals system intelligence, active computation, selected state, focus, and causal flow.

| Token | Hex | Usage |
| --- | --- | --- |
| `logic-blue-050` | `#D9ECFF` | Rare text on blue-filled states |
| `logic-blue-200` | `#8FC4FF` | Glow edge, secondary chart line |
| `logic-blue-400` | `#4A9CFF` | Hover state, active icon stroke |
| `logic-blue-500` | `#2D84F7` | Primary active status, focus ring, selected border |
| `logic-blue-700` | `#1A58B8` | Dense fills, pressed state |
| `logic-blue-900` | `#0C2E63` | Deep background tint behind active modules |

### Functional Status Colors

Velarix defaults to neutral or Logic Blue states. Only use success, warning, and danger when a real operational state exists.

| Token | Hex | Usage |
| --- | --- | --- |
| `success` | `#3CB179` | Confirmed success only |
| `warning` | `#D9A441` | Risk, review required, degraded state |
| `danger` | `#E35D6A` | Failure, destructive action, critical alert |

### Color Usage Rules

- `background` must cover the entire viewport.
- `surface` is the default container color. Do not invent extra gray variants unless there is a hierarchy reason.
- `primary-accent-chrome` is not a fill color for large blocks. It is a precision accent.
- `logic-blue-500` is the default focus and selected-state color.
- Large gradient fields are allowed only in hero or data-visual moments, and must stay inside the dark palette.
- Pure white `#FFFFFF` and pure black `#000000` should not be used directly except for transparent overlays.

## 4. Overlay, Opacity, and Depth

Depth is created with border contrast, layered surfaces, and extremely restrained overlays.

| Token | Value | Usage |
| --- | --- | --- |
| `overlay-soft` | `rgba(255, 255, 255, 0.05)` | Hover wash, active panel sheen |
| `overlay-strong` | `rgba(255, 255, 255, 0.08)` | Selected surfaces only |
| `scrim` | `rgba(4, 7, 10, 0.72)` | Modal and drawer background |
| `focus-ring` | `0 0 0 1px #2D84F7, 0 0 0 3px rgba(45, 132, 247, 0.18)` | Keyboard focus |

Rules:

- Default hover treatment is a `0.05` white overlay over the base surface.
- Shadows are secondary. Borders do the real work.
- If a component looks soft, it is off-brand.

## 5. Typography

### Font Stack

- Primary Sans-Serif: `Inter`
- Accent Serif: `Lora`
- Sans fallback: `ui-sans-serif, system-ui, sans-serif`
- Final sans fallback: `ui-sans-serif, system-ui, sans-serif`
- Final serif fallback: `ui-serif, Georgia, serif`

### Typography Rules

- Use `Inter` for all technical UI, buttons, body copy, navigation, and primary headings.
- Use `Lora` italic sparingly for editorial accent words in headlines, especially transition words such as `for`, `is`, and `the`.
- Use optical hierarchy through size, weight, and tracking, not through many colors.
- Body copy should stay left-aligned.
- Avoid fully justified text.
- Avoid all-caps for long labels. All-caps are reserved for micro labels and metadata.
- H1 must use `Inter`, tighter tracking, and `font-semibold`.
- Featured accent words must use `Lora`, italic, and `font-medium`.
- Body copy must use `Inter`, relaxed leading, and muted supporting color.

### Type Scale

| Style | Size | Line Height | Weight | Tracking | Usage |
| --- | --- | --- | --- | --- | --- |
| `H1` | `56px` | `60px` | `600` | `-0.04em` | Hero headline, single focal statement |
| `H2` | `40px` | `46px` | `600` | `-0.03em` | Section headline |
| `H3` | `30px` | `36px` | `600` | `-0.022em` | Subsection headline |
| `H4` | `24px` | `30px` | `600` | `-0.018em` | Card title, modal title |
| `H5` | `18px` | `24px` | `600` | `-0.012em` | Compact section title |
| `H6` | `15px` | `20px` | `600` | `-0.008em` | Dense UI heading |
| `Body` | `16px` | `26px` | `400` | `-0.006em` | Standard paragraph copy |
| `Body Small` | `14px` | `22px` | `400` | `-0.004em` | Secondary explanatory copy |
| `Label` | `12px` | `16px` | `500` | `0.04em` | Eyebrows, micro labels, field labels |
| `Mono Data` | `13px` | `18px` | `500` | `0.01em` | Optional numeric readouts only |

### Type Application Rules

- One page should usually contain only 3 active text levels at once.
- H1 should be rare. Most pages should start at H2 or H3.
- Paragraph width target: `56ch` to `72ch`.
- Label text may use uppercase; headlines and body text must not.
- Editorial serif accents are emphasis devices, not alternate paragraph styling.
- Never set full paragraphs or long UI labels in `Lora`.

## 6. Spacing and Layout

### Base Grid

- Base unit: `4px`
- Primary layout rhythm: `8px`, `12px`, `16px`, `24px`, `32px`, `48px`, `64px`

### Container Widths

- Max readable text width: `720px`
- Standard content container: `1200px`
- Wide visualization container: `1360px`

### Section Spacing

- Desktop major sections: `96px` top and bottom padding minimum
- Tablet major sections: `72px`
- Mobile major sections: `56px`

## 7. The Velarix Look

This is the mechanical language every component must share.

### Borders

- Default border: `1px solid #242C36`
- Nested dividers: `1px solid #1A212B`
- No dashed borders in production UI unless representing an empty upload or insertion state

### Radius

- Inputs, buttons, chips: `4px`
- Cards, panels, modals, tables, code blocks: `6px`
- Large editorial callouts: `6px`
- Avoid `8px+` unless the element is explicitly soft by purpose, which should be rare
- No pill buttons for the primary site UI

### Surfaces

- Surfaces are flat blocks with disciplined layering
- Use `surface`, `surface-elevated`, and `surface-muted` instead of inventing arbitrary fills
- Every card must have a border even when it also has a background contrast

### Buttons

- Primary button: dark base with chrome or Logic Blue emphasis, never candy-colored
- Secondary button: surface fill with border and subtle hover overlay
- Ghost button: transparent with borderless structure, only where density requires it
- Padding must feel compact, not oversized

### Inputs

- Inputs must look technical: dark fill, clear border, hard focus ring
- Placeholder copy is muted and minimal
- Validation should use border and text change before using heavy fills

### Icons

- Use `lucide-react` only
- Default icon size: `16px`, `18px`, or `20px`
- Keep icon stroke visually crisp and aligned with text baseline
- Use icons to clarify action, not to decorate headings

### Data Visualization

- Charts should default to neutral grays plus Logic Blue
- Use one accent color per chart unless comparison genuinely requires more
- Grids and axes must be understated

## 8. Motion Principles

Motion must communicate causality and state change.

- Default durations: `140ms`, `180ms`, `240ms`
- Large section reveals: `320ms` to `480ms`
- Preferred easing: sharp ease-out for entry, linear or near-linear for data-linked motion
- Avoid bounce, springiness, and playful overshoot in marketing surfaces

For "Causal Collapse" visuals:

- Motion should imply compression, convergence, and deterministic resolution
- Prefer opacity, transform, clipping, and layout transitions over blur-heavy effects
- Motion should feel computational, not cinematic

## 9. Imagery and Illustration

- Prefer diagrams, product frames, abstract data forms, and structural line work
- Avoid stock-photo optimism
- Avoid people unless there is a real documentary reason
- Background illustration should stay low-contrast and architectural

## 10. Copy Voice and Tone

Velarix speaks as the adult in the room.

### Core Traits

- Deterministic
- Precise
- Technical
- Calm
- Unimpressed by noise
- Trustworthy without sounding corporate

### Writing Rules

- Prefer short declarative sentences
- Lead with the conclusion, then the mechanism
- Use exact terms instead of metaphors when explaining capability
- Do not overclaim
- Do not use hype language, swagger, or startup clichés
- Avoid filler phrases like "seamless", "revolutionary", "game-changing", "unleash", and "next-level"

### Brand Voice Examples

| Weak | Correct Velarix Voice |
| --- | --- |
| "We supercharge your AI workflows." | "Velarix gives AI systems a causal memory they can query and verify." |
| "Experience the future of intelligent infrastructure." | "Run inference against state that preserves dependency, sequence, and proof." |
| "Powerful, flexible, scalable." | "Built for high-volume, auditable, low-latency decision systems." |

## 11. Accessibility Rules

- Minimum text contrast must meet WCAG AA, even in dark mode
- Focus states must be visible without color ambiguity
- Motion-heavy sections must respect reduced-motion preferences
- Information must never rely on blue alone; pair color with label, icon, or structure

## 12. Implementation Notes

- Encode these values as design tokens first, then consume them in Tailwind theme variables and shadcn component overrides
- If a new component conflicts with this spec, the component changes, not the spec
- Any future light theme exploration is out of scope and should not influence current implementation choices
