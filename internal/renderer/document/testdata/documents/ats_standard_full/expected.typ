// Import the rendercv function and all the refactored components
#import "@preview/rendercv:0.3.0": *

// Apply the rendercv template with custom configuration
#show: rendercv.with(
  name: "Alice Chen",
  title: "Alice Chen - CV",
  footer: context { [#emph[Alice Chen -- #str(here().page())\/#str(counter(page).final().first())]] },
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


= Alice Chen

#connections(
  [#connection-with-icon("location-dot")[San Francisco, CA]],
  [#link("mailto:alice.chen@email.com", icon: false, if-underline: false, if-color: false)[#connection-with-icon("envelope")[alice.chen\@email.com]]],
  [#link("tel:+1-415-555-0142", icon: false, if-underline: false, if-color: false)[#connection-with-icon("phone")[(415) 555-0142]]],
  [#link("https://alicechen.dev/", icon: false, if-underline: false, if-color: false)[#connection-with-icon("link")[alicechen.dev]]],
  [#link("https://linkedin.com/in/alicechen", icon: false, if-underline: false, if-color: false)[#connection-with-icon("linkedin")[alicechen]]],
  [#link("https://github.com/alicechen", icon: false, if-underline: false, if-color: false)[#connection-with-icon("github")[alicechen]]],
)


== Summary

Senior software engineer with 8 years of experience in distributed systems and cloud infrastructure. Led teams of 5-12 engineers at two Fortune 500 companies.

== Experience

#regular-entry(
  [
    #strong[Stripe], Staff Software Engineer

    - Designed and implemented real-time fraud detection pipeline processing 50M+ transactions daily with 99.99\% uptime

    - Led migration of payment processing infrastructure from monolith to microservices, reducing deployment time by 80\%

    - Mentored 6 engineers across 2 teams, with 3 promoted to senior level within 18 months

  ],
  [
    San Francisco, CA

    Mar 2021 – present

    

    4 years 1 month

  ],
)

#regular-entry(
  [
    #strong[Google], Senior Software Engineer

    - Built distributed caching layer for Google Cloud Storage serving 100K+ QPS with sub-millisecond latency

    - Reduced infrastructure costs by \$2.4M annually through optimization of resource allocation algorithms

    - Contributed to open-source gRPC framework with 40K+ GitHub stars

  ],
  [
    Mountain View, CA

    June 2018 – Feb 2021

    

    2 years 9 months

  ],
)

#regular-entry(
  [
    #strong[Amazon Web Services], Software Development Engineer

    - Developed auto-scaling algorithms for EC2 fleet management across 3 AWS regions

    - Implemented end-to-end monitoring dashboard used by 200+ internal teams

  ],
  [
    Seattle, WA

    July 2016 – May 2018

    

    1 year 11 months

  ],
)

== Education

#education-entry(
  [
    #strong[Stanford University], Computer Science

    - Focus: Distributed Systems and Database Theory

  ],
  [
    Stanford, CA

    Sept 2014 – June 2016

  ],
  degree-column: [
    #strong[MS]
  ],
)

#education-entry(
  [
    #strong[UC Berkeley], Electrical Engineering and Computer Science

    - GPA: 3.92\/4.00, Magna Cum Laude

  ],
  [
    Berkeley, CA

    Aug 2010 – May 2014

  ],
  degree-column: [
    #strong[BS]
  ],
)

== Skills

#strong[Languages:] Python, Go, Java, C++, TypeScript, SQL

#strong[Infrastructure:] Kubernetes, Docker, Terraform, AWS, GCP

#strong[Databases:] PostgreSQL, Redis, DynamoDB, BigQuery, Cassandra

#strong[Tools:] Git, CI\/CD, Prometheus, Grafana, Datadog

== Certifications

- AWS Solutions Architect Professional (2023)

- Google Cloud Professional Cloud Architect (2022)

- Certified Kubernetes Administrator (2021)
