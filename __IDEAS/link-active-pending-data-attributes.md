# Link Active And Pending Data Attributes

## Idea

Add active and pending data attributes to `Link`.

Preferred public behavior:

```html
<a data-vorma-active="true" data-vorma-pending="true"></a>
```

## Why

Users should be able to style active and pending links with plain CSS without
Vorma adding class-name APIs, render props, or styling opinions.

## Notes

- Active state can likely be computed from current route/location.
- Pending state needs the current navigation target href, not just global
  `isNavigating`.
- Avoid making every link subscribe to a broad object that changes on unrelated
  route data updates.
