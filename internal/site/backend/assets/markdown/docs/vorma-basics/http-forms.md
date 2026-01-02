---
title: Working With Traditional HTTP Forms
description: How to use traditional HTTP forms with Vorma
order: 990
---

Typically, Vorma API endpoints will operate with serializable JSON data.
However, sometimes (_e.g._, when you need file uploads) you will want to use a
normal HTTP form. Vorma supports this through the usage of `FormData` and works
with both `application/x-www-form-urlencoded` and `multipart/form-data` forms.

### Backend

On the backend, you will want to:

- Use `vorma.FormData` as the `I` generic to your action context, which will
  cause your frontend API client to expect a `FormData` instance as the input
  type.
- Call `ParseMultipartForm` on the http request, as you would in any Go handler,
  and then do whatever you want.

It will probably look something like this:

```go
var _ = NewMutation("/form", func(c *ActionCtx[vorma.FormData]) (string, error) {
	r := c.Request() // standard *http.Request
	if err := r.ParseMultipartForm(5 << 20); err != nil {
		return nil, errors.New("malformed input")
	}
	vals := r.MultipartForm.Value
	files := r.MultipartForm.File
	// upload the file to an s3 bucket or whatever
	return "looks good, thanks", nil
})
```

If you need to learn more about how to operate with forms in Go, check out the
Go docs themselves. There's nothing special or magic happening here, so you can
use anything in the Go standard library (or any other compatible form parsing
library you want).

### Frontend

Then, on the frontend, all you need to do is populate a `FormData` instance
(most likely, but not necessarily, through an actual http `form` element), and
then submit to the Vorma API like any other endpoint, like so:

```tsx
<form
	onSubmit={async (e) => {
		e.preventDefault();
		await api.mutate({
			pattern: "/form",
			// must be a FormData instance to satisfy TS compiler
			input: new FormData(e.currentTarget),
		});
	}}
>
	<input name="username" required />
	<input name="avatar" type="file" />
	<button>Submit</button>
</form>
```

In the future, we may provide a first-party `Form` component that reduces
boilerplate (TBD).
