# Search

## Basic Search

Enter your search query in the search box and press Enter.

For the public deployment, start at `https://x.scour.li/`.

## Filtering Results

### By Engine

Select specific engines from the dropdown, use the `engines` parameter, or add bang shortcuts directly to the query.

### By Duration

Use duration filters: any, under 10 minutes, 10–30 minutes, or over 30 minutes.

### By Quality

Filter by quality: any, 720p, 1080p, or 4K. Available values depend on metadata
returned by each source.

### Sorting

Sort by relevance, duration, views, or quality. A source may omit metadata used
by a filter or sort option.

## Search Operators

| Operator | Example | Description |
|----------|---------|-------------|
| Bang | `!ph amateur` | Search a specific engine directly |
| Multiple bangs | `!ph !rt amateur` | Search multiple engines in one query |
| Quotes | `"exact phrase"` | Preserve an exact phrase as a single search term |
| Minus | `-term` | Exclude results containing that term |

Short and full bang names are both supported for common engines, such as `!ph` and `!pornhub`.
