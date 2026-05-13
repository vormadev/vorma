# Vorma Remix Component Matrix

Living component inventory for `kit/lab/design/components/remix`.

Status cells are intentionally blank until they are re-audited. Do not add
ambiguous status values or extra matrix columns.

Do not move onto starting new components while others are still in
`Fully built: WIP` status, otherwise we will have a bunch of half-finished
components. For now, it's fine to leave them in `Fully tested: WIP` state,
though.

An empty status cell means the work has not started.

`WIP` means the work has started. See the component's individual notes file for
detailed status. Every `WIP` row must have a matching
`<component-name>.notes.md` file using the component name in kebab-case.

`yes` means the column's criterion has been satisfied.

`Fully built` means the component is fully built out with WAI-ARIA compliant
behavior, all long-term intended public APIs, and semantics based on thorough
research of existing mature component libraries, Open UI, and relevant
accessibility guidelines.

`Fully tested` means the component has enough automated coverage to prove the
public API, accessibility behavior, keyboard and pointer behavior, controlled
and uncontrolled state, styling escape hatches, and relevant edge cases.

| Component        | Fully built | Fully tested |
| ---------------- | ----------- | ------------ |
| Accordion        |             |              |
| Alert            |             |              |
| AlertDialog      |             |              |
| AnimatedImage    |             |              |
| Animation        |             |              |
| AspectRatio      |             |              |
| Autocomplete     |             |              |
| Avatar           |             |              |
| Badge            |             |              |
| Box              |             |              |
| Blockquote       |             |              |
| Breadcrumbs      |             |              |
| Button           | yes         | WIP          |
| ButtonGroup      |             |              |
| Calendar         |             |              |
| Callout          |             |              |
| Card             |             |              |
| Carousel         |             |              |
| Checkbox         | yes         | WIP          |
| CheckboxGroup    | yes         | WIP          |
| Chip             |             |              |
| CloseButton      |             |              |
| Code             |             |              |
| CodeBlock        |             |              |
| ColorPicker      |             |              |
| Combobox         |             |              |
| Command          |             |              |
| Container        |             |              |
| ContextMenu      |             |              |
| CopyButton       |             |              |
| DatePicker       |             |              |
| DescriptionList  |             |              |
| Dialog           |             |              |
| Drawer           |             |              |
| Editable         |             |              |
| EmptyState       |             |              |
| Field            | yes         | WIP          |
| Fieldset         |             |              |
| FileInput        |             |              |
| Flex             |             |              |
| Form             |             |              |
| FormatBytes      |             |              |
| FormatDate       |             |              |
| FormatNumber     |             |              |
| Grid             |             |              |
| Heading          |             |              |
| HoverCard        |             |              |
| Icon             |             |              |
| IconButton       |             |              |
| Image            |             |              |
| ImageComparer    |             |              |
| Include          |             |              |
| Input            | yes         | WIP          |
| Kbd              | yes         | WIP          |
| Label            |             |              |
| Link             |             |              |
| List             |             |              |
| Listbox          | yes         | WIP          |
| LiveRegion       |             |              |
| Menu             | yes         | WIP          |
| Menubar          |             |              |
| Meter            |             |              |
| NavigationMenu   |             |              |
| NativeSelect     |             |              |
| NumberField      |             |              |
| OTPField         |             |              |
| Pagination       |             |              |
| PasswordField    |             |              |
| Popover          |             |              |
| Portal           |             |              |
| Presence         |             |              |
| Progress         |             |              |
| ProgressRing     |             |              |
| QRCode           |             |              |
| RadioGroup       | yes         | WIP          |
| Rating           |             |              |
| RelativeTime     |             |              |
| ResizeObserver   |             |              |
| ScrollArea       |             |              |
| SearchField      |             |              |
| Select           | yes         | WIP          |
| SegmentedControl |             |              |
| Separator        | yes         | WIP          |
| Skeleton         |             |              |
| SkipNav          |             |              |
| Slider           |             |              |
| Spinner          |             |              |
| SplitPanel       |             |              |
| Stack            |             |              |
| Stat             |             |              |
| Stepper          |             |              |
| Surface          |             |              |
| Switch           | yes         | WIP          |
| Table            |             |              |
| Tabs             |             |              |
| Tag              |             |              |
| TagsInput        |             |              |
| Text             |             |              |
| Textarea         | yes         | WIP          |
| Timeline         |             |              |
| Toast            |             |              |
| Toggle           |             |              |
| ToggleGroup      |             |              |
| Toolbar          |             |              |
| Tooltip          |             |              |
| Tree             |             |              |
| VisuallyHidden   | yes         | WIP          |
