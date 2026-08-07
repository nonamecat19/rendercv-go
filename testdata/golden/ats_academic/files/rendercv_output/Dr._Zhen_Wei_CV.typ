// Import the rendercv function and all the refactored components
#import "@preview/rendercv:0.3.0": *

// Apply the rendercv template with custom configuration
#show: rendercv.with(
  name: "Dr. Zhen Wei",
  title: "Dr. Zhen Wei - CV",
  footer: context { [#emph[Dr. Zhen Wei -- #str(here().page())\/#str(counter(page).final().first())]] },
  top-note: [ #emph[Last updated in Mar 2025] ],
  locale-catalog-language: "en",
  text-direction: ltr,
  page-size: "us-letter",
  page-top-margin: 0.7in,
  page-bottom-margin: 0.7in,
  page-left-margin: 0.7in,
  page-right-margin: 0.7in,
  page-show-footer: true,
  page-show-top-note: true,
  colors-body: rgb(0, 0, 0),
  colors-name: rgb(0, 79, 144),
  colors-headline: rgb(0, 79, 144),
  colors-connections: rgb(0, 79, 144),
  colors-section-titles: rgb(0, 79, 144),
  colors-links: rgb(0, 79, 144),
  colors-footer: rgb(128, 128, 128),
  colors-top-note: rgb(128, 128, 128),
  typography-line-spacing: 0.6em,
  typography-alignment: "justified",
  typography-date-and-location-column-alignment: right,
  typography-font-family-body: "Source Sans 3",
  typography-font-family-name: "Source Sans 3",
  typography-font-family-headline: "Source Sans 3",
  typography-font-family-connections: "Source Sans 3",
  typography-font-family-section-titles: "Source Sans 3",
  typography-font-size-body: 10pt,
  typography-font-size-name: 30pt,
  typography-font-size-headline: 10pt,
  typography-font-size-connections: 10pt,
  typography-font-size-section-titles: 1.4em,
  typography-small-caps-name: false,
  typography-small-caps-headline: false,
  typography-small-caps-connections: false,
  typography-small-caps-section-titles: false,
  typography-bold-name: true,
  typography-bold-headline: false,
  typography-bold-connections: false,
  typography-bold-section-titles: true,
  links-underline: false,
  links-show-external-link-icon: false,
  header-alignment: center,
  header-photo-width: 3.5cm,
  header-space-below-name: 0.7cm,
  header-space-below-headline: 0.7cm,
  header-space-below-connections: 0.7cm,
  header-connections-hyperlink: true,
  header-connections-show-icons: true,
  header-connections-display-urls-instead-of-usernames: false,
  header-connections-separator: "",
  header-connections-space-between-connections: 0.5cm,
  section-titles-type: "with_partial_line",
  section-titles-line-thickness: 0.5pt,
  section-titles-space-above: 0.5cm,
  section-titles-space-below: 0.3cm,
  sections-allow-page-break: true,
  sections-space-between-text-based-entries: 0.3em,
  sections-space-between-regular-entries: 1.2em,
  entries-date-and-location-width: 4.15cm,
  entries-side-space: 0.2cm,
  entries-space-between-columns: 0.1cm,
  entries-allow-page-break: false,
  entries-short-second-row: true,
  entries-degree-width: 1cm,
  entries-summary-space-left: 0cm,
  entries-summary-space-above: 0cm,
  entries-highlights-bullet:  "•" ,
  entries-highlights-nested-bullet:  "•" ,
  entries-highlights-space-left: 0.15cm,
  entries-highlights-space-above: 0cm,
  entries-highlights-space-between-items: 0cm,
  entries-highlights-space-between-bullet-and-text: 0.5em,
  date: datetime(
    year: 2025,
    month: 3,
    day: 5,
  ),
)


= Dr. Zhen Wei

  #headline([Associate Professor of Computer Science])

#connections(
  [#connection-with-icon("location-dot")[Zurich, Switzerland]],
  [#link("mailto:zhen.wei@email.com", icon: false, if-underline: false, if-color: false)[#connection-with-icon("envelope")[zhen.wei\@email.com]]],
  [#link("https://zhenwei.ch/", icon: false, if-underline: false, if-color: false)[#connection-with-icon("link")[zhenwei.ch]]],
  [#link("https://scholar.google.com/citations?user=DEF456GHI", icon: false, if-underline: false, if-color: false)[#connection-with-icon("graduation-cap")[Google Scholar]]],
  [#link("https://orcid.org/0000-0003-9876-5432", icon: false, if-underline: false, if-color: false)[#connection-with-icon("orcid")[0000-0003-9876-5432]]],
  [#link("https://github.com/zhenwei", icon: false, if-underline: false, if-color: false)[#connection-with-icon("github")[zhenwei]]],
)


== Education

#education-entry(
  [
    #strong[ETH Zurich], Computer Science

    - Thesis: Scalable Graph Neural Networks for Molecular Discovery

  ],
  [
    Zurich, Switzerland

    Sept 2012 – Mar 2017

  ],
  degree-column: [
    #strong[PhD]
  ],
)

#education-entry(
  [
    #strong[Tsinghua University], Computer Science

  ],
  [
    Beijing, China

    Sept 2010 – July 2012

  ],
  degree-column: [
    #strong[MS]
  ],
)

#education-entry(
  [
    #strong[Peking University], Mathematics

    - GPA: 3.9\/4.0

  ],
  [
    Beijing, China

    Sept 2006 – July 2010

  ],
  degree-column: [
    #strong[BS]
  ],
)

== Academic Positions

#regular-entry(
  [
    #strong[ETH Zurich], Associate Professor

    - Lead Computational Science Lab with 12 PhD students and 4 postdocs

    - Teaching: Advanced Machine Learning (300+ students), Graph Neural Networks (150+ students)

  ],
  [
    Zurich, Switzerland

    Sept 2022 – present

  ],
)

#regular-entry(
  [
    #strong[ETH Zurich], Assistant Professor

    - Established research group, secured CHF 3M+ in competitive funding

  ],
  [
    Zurich, Switzerland

    Sept 2017 – Aug 2022

  ],
)

#regular-entry(
  [
    #strong[Stanford University], Postdoctoral Researcher

    - Collaborated with Prof. Jure Leskovec on graph representation learning

  ],
  [
    Stanford, CA

    Apr 2017 – Aug 2017

  ],
)

== Publications

#regular-entry(
  [
    #strong[Equivariant Graph Neural Networks for 3D Molecular Generation]

    #emph[Zhen Wei], Anna Mueller

    #link("https://doi.org/10.1038/s42256-024-0001")[10.1038\/s42256-024-0001] (Nature Machine Intelligence)

  ],
  [
    Jan 2024

  ],
)

#regular-entry(
  [
    #strong[Scalable Message Passing on Large Graphs via Stochastic Training]

    #emph[Zhen Wei], Li Zhang, Marco Rossi

    #link("https://doi.org/10.5555/icml.2023.001")[10.5555\/icml.2023.001] (ICML 2023)

  ],
  [
    July 2023

  ],
)

#regular-entry(
  [
    #strong[Self-Supervised Pre-Training for Molecular Property Prediction]

    David Kim, #emph[Zhen Wei]

    #link("https://doi.org/10.5555/neurips.2022.001")[10.5555\/neurips.2022.001] (NeurIPS 2022)

  ],
  [
    Dec 2022

  ],
)

#regular-entry(
  [
    #strong[Geometric Deep Learning on Protein Surfaces]

    #emph[Zhen Wei], Sarah Johnson

    #link("https://doi.org/10.1126/science.2022")[10.1126\/science.2022] (Science)

  ],
  [
    June 2022

  ],
)

#regular-entry(
  [
    #strong[Graph Transformers with Spectral Attention]

    #emph[Zhen Wei]

    #link("https://doi.org/10.5555/iclr.2022.001")[10.5555\/iclr.2022.001] (ICLR 2022)

  ],
  [
    Apr 2022

  ],
)

== Grants

- Swiss National Science Foundation Eccellenza Grant (CHF 1.8M, 2023-2028)

- ERC Starting Grant (EUR 1.5M, 2020-2025)

- ETH Research Grant (CHF 500K, 2018-2021)

== Awards

- ELLIS Fellow (2023)

- MIT Technology Review Innovators Under 35 Europe (2022)

- ETH Zurich Latsis Prize for Outstanding Young Researcher (2021)

- ICML Best Paper Award (2019)

== Service

- Area Chair: NeurIPS (2022, 2023, 2024), ICML (2023, 2024), ICLR (2024)

- Associate Editor: IEEE TPAMI (2023-present)

- Program Committee: KDD, AAAI, IJCAI, WWW

== Skills

#strong[Languages:] Python, C++, Julia, MATLAB

#strong[ML Frameworks:] PyTorch, JAX, PyG, DGL

#strong[Scientific Computing:] NumPy, SciPy, RDKit, Open Babel, GROMACS
