# Frontend framework decision

## Status

Accepted on 2026-09-03.

## What is being selected

There are three separate choices that are often called a “UI framework”:

1. An application/component framework (React, Vue, Svelte, or server HTML).
2. A styling system (Tailwind CSS, CSS modules, or handwritten CSS).
3. A component source/library (shadcn/ui, Material UI, Chakra UI, and others).

Beautiful results still require coherent typography, spacing, color, responsive
behavior, accessibility, and states. A library accelerates that work but does
not choose a product design for us.

## Project-specific criteria

- Excellent upload progress, gallery, dialogs, forms, toasts, and media-player UI.
- Responsive and accessible components that are easy to restyle.
- A clean API boundary: Go remains the only backend.
- Straightforward Docker development and static production output.
- Low conceptual overhead for a small project.
- Healthy documentation and sufficient component examples.

## Application framework options

| Option | Strengths here | Costs here | Fit |
| --- | --- | --- | --- |
| React + Vite + TypeScript | Largest component ecosystem; shadcn/ui's first-class Vite support; strong tooling; static output talks directly to Go | JSX and React state/effect concepts add overhead; many ecosystem choices | Best access to polished components |
| Vue + Vite + TypeScript | HTML-like single-file components; low conceptual overhead; good UI ecosystems such as PrimeVue and Vuetify | Smaller ecosystem than React; official shadcn CLI does not currently list Vue | Strong ease/ecosystem balance |
| Svelte/SvelteKit | Concise components, little boilerplate, pleasant reactivity; static adapter available | Smaller ecosystem; some attractive component kits are community ports; SvelteKit server features would overlap Go | Best for compact custom UI |
| Go templates + HTMX | One language/runtime on the server; very little JavaScript; simple deployment | Rich upload interactions and reusable polished components take more custom work; full page/server coupling | Best for minimum stack, not maximum UI leverage |
| Next.js | Huge React ecosystem, routing and full-stack features, strong Docker support | Its server/rendering model duplicates responsibilities already assigned to Go; more runtime and caching concepts | Capable, but unnecessary initially |
| Nuxt | Approachable Vue full-stack framework and flexible Node/static deployment | Like Next, much of its server capability overlaps the Go service | Capable, but unnecessary initially |

## Styling and component choices

### Tailwind CSS

Tailwind provides small utility classes, responsive variants, design tokens, and
zero-runtime generated CSS. It makes a custom visual system fast to iterate and
pairs naturally with shadcn/ui. The tradeoff is visually dense class attributes;
repeated patterns should become components rather than indiscriminate `@apply`
wrappers.

### shadcn/ui

shadcn/ui copies component source into the project instead of hiding it behind a
package API. That gives us control over appearance and behavior—useful for a
distinctive product—and its current official scaffolding supports React-oriented
targets including Vite and Next.js. The cost is ownership: we maintain the copied
components and must test our customizations.

### Batteries-included suites

Material UI, Chakra UI, PrimeVue, and similar suites provide broad component
coverage and can be faster when a recognizable design system is acceptable.
They are less attractive if the goal is a strongly custom aesthetic, but remain
valid if “ship quickly” outweighs visual ownership.

## Decision

Use **React + Vite + TypeScript + Tailwind CSS + shadcn/ui**.

Why it fits:

- Vite can build a static client; Go remains the only application backend.
- The React ecosystem offers the widest selection for uploads, media, forms, and
  accessible primitives.
- shadcn/ui supplies polished building blocks whose source we can adapt rather
  than fighting a fixed theme.
- TypeScript makes the API contract visible and catches common client mistakes.

This is not the smallest conceptual surface, but the polished component ecosystem
and clean static-client/Go-API boundary are the preferred tradeoff.

## Questions for the decision session

1. Is the desired visual direction highly custom, or is a themed component suite
   acceptable?
2. Is adopting a mainstream frontend ecosystem worth the additional stack complexity?
3. Which matters more: the smallest codebase or the fastest access to polished,
   interactive components?
4. Should the public clip page work with almost no JavaScript?
5. Are there reference sites or screenshots that define “nicer” for this project?

## Decision record template

- **Decision:** Accepted
- **Date:** 2026-09-03
- **Chosen option:** React, Vite, TypeScript, Tailwind CSS, and shadcn/ui
- **Reasons:** Polished customizable components, strong ecosystem, static client,
  and a clear boundary with the Go backend.
- **Rejected alternatives:** Vue and Svelte have smaller applicable component
  ecosystems; Go templates/HTMX offer less leverage for this interactive UI;
  Next.js/Nuxt add server responsibilities already owned by Go.
- **Revisit when:** Requirements or deployment constraints materially change

## Official references checked

- React describes its component model at <https://react.dev/learn>.
- Vite documentation is at <https://vite.dev/guide/>.
- Tailwind's utility and responsive model is documented at
  <https://tailwindcss.com/docs/styling-with-utility-classes> and
  <https://tailwindcss.com/docs/responsive-design>.
- shadcn/ui's currently supported setup targets are listed at
  <https://ui.shadcn.com/docs/installation>.
- Svelte's official adapters are listed at <https://svelte.dev/packages>.
- Next.js documents Docker and static-export deployment at
  <https://nextjs.org/docs/app/getting-started/deploying>.
- Nuxt documents Node and static deployment at
  <https://nuxt.com/docs/3.x/getting-started/deployment>.
