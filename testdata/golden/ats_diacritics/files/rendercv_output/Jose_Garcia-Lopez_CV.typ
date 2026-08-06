// Import the rendercv function and all the refactored components
#import "@preview/rendercv:0.3.0": *

// Apply the rendercv template with custom configuration
#show: rendercv.with(
  name: "Jose Garcia-Lopez",
  title: "Jose Garcia-Lopez - CV",
  footer: context { [#emph[Jose Garcia-Lopez -- #str(here().page())\/#str(counter(page).final().first())]] },
  top-note: [ #emph[Last updated in Aug 2026] ],
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
    year: 2026,
    month: 8,
    day: 6,
  ),
)


= Jose Garcia-Lopez

  #headline([Ingeniero de Software Senior])

#connections(
  [#connection-with-icon("location-dot")[Barcelona, Spain]],
  [#link("mailto:jose.garcia@email.com", icon: false, if-underline: false, if-color: false)[#connection-with-icon("envelope")[jose.garcia\@email.com]]],
  [#link("tel:+34-612-34-56-78", icon: false, if-underline: false, if-color: false)[#connection-with-icon("phone")[612 34 56 78]]],
  [#link("https://linkedin.com/in/josegarcia", icon: false, if-underline: false, if-color: false)[#connection-with-icon("linkedin")[josegarcia]]],
  [#link("https://github.com/josegarcia", icon: false, if-underline: false, if-color: false)[#connection-with-icon("github")[josegarcia]]],
)


== Experience

#regular-entry(
  [
    #strong[Glovo], Senior Backend Engineer

    - Architected microservices platform handling 5M+ daily orders across 25 countries

    - Led migration from monolithic PHP application to Go microservices

  ],
  [
    Barcelona, Spain

    Mar 2021 – present

    

    5 years 6 months

  ],
)

#regular-entry(
  [
    #strong[Banco Santander], Software Engineer

    - Developed real-time transaction processing system for retail banking

  ],
  [
    Madrid, Spain

    Sept 2018 – Feb 2021

    

    2 years 6 months

  ],
)

== Education

#education-entry(
  [
    #strong[Universidad Politecnica de Madrid], Computer Science

  ],
  [
    Madrid, Spain

    Sept 2016 – June 2018

  ],
  degree-column: [
    #strong[MS]
  ],
)

#education-entry(
  [
    #strong[Universitat de Barcelona], Mathematics

    - GPA: 9.2\/10.0, Cum Laude

  ],
  [
    Barcelona, Spain

    Sept 2012 – June 2016

  ],
  degree-column: [
    #strong[BS]
  ],
)

== Skills

#strong[Languages:] Go, Python, Java, PHP, SQL

#strong[Infrastructure:] Kubernetes, Docker, AWS, Terraform
