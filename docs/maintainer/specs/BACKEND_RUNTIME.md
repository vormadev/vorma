# Backend Runtime

## What The Backend Runtime Is

The backend runtime is the server-side request handler for a Vorma app. It is the part an
application mounts into its normal HTTP server when it wants Vorma to answer Vorma-owned
requests.

At request time, the runtime combines three sources of truth:

- The app declaration: views, resources, middleware, shared state, and document defaults.
- The current runtime assets: manifest, client build id, view modules, CSS, public file
  map, and critical CSS.
- The incoming HTTP request.

From those inputs, the runtime decides whether the request is an API call, a public asset
request, a full view document request, or a view-data request for the browser client.

The runtime's central job is to turn a request into a coherent HTTP response. For views,
that means one response that agrees about the matched patterns, path captures, server view
data, document head, client assets, response effects, and client build identity. For
resources, it means one response that reflects the matched resource handler, parsed input,
response effects, and current build identity.

The runtime is not the whole application server. It does not own the application's outer
HTTP stack, non-Vorma endpoints, deployment adapter, database, or background workers. It
is the Vorma-owned request-time boundary inside the larger server.

The runtime is also not the build system or the browser router. It consumes the current
build output and emits the data the browser needs, but it does not decide how Vite builds
modules or how the client router commits navigation state.

## What The Backend Runtime Owns

The backend runtime owns Vorma's request-time semantics on the server.

It owns deciding whether an incoming request belongs to Vorma at all. If the request is
under the API mount, it is a resource candidate. If it targets a manifest-backed public
asset, it is an asset request. If it is a `GET` or `HEAD` request for a declared view, it
is a view request. Requests outside those boundaries belong to the surrounding application
server.

It owns executing server-side Vorma work for a request: middleware, resource handlers,
view handlers, document building, response-effect collection, and public asset lookup.
That work must be combined into one HTTP response with consistent status, headers,
cookies, redirects, head state, view payload, and client build identity.

It owns the server half of the view contract. A view can be delivered as HTML for first
load or as JSON view data for client navigation/revalidation, but both responses must
describe the same view meaning: the same matched patterns, captures, view results, view
errors, head state, and client asset requirements.

It owns protecting the browser from incompatible view data. If a view-data request was
made by a stale client build, the runtime must report build skew instead of returning
normal view payload data.

It owns preserving ordinary HTTP behavior at the boundary. Request body limits,
method-not-allowed responses, `HEAD` behavior, not-found responses, static asset headers,
and redirect status/header behavior should all look like normal HTTP to the caller.

## What The Backend Runtime Does Not Own

## App Declaration To Runtime Host

## Request Ownership And Dispatch

## Runtime Asset Snapshot

## Client Build Identity

## Build Skew

## View Requests

## Nested View Matching

## View Handler Execution

## Document Rendering

## View Data Requests

## Document And View Data Equivalence

## View Payload

## Head State

## Client Asset Requirements

## Response Effects

## Response Effect Merging

## Redirects

## Resources

## Resource Input Parsing

## Form Data

## Middleware

## Cancellation

## Error Surfaces

## Public Static Assets

## Request Body Limits

## HEAD Requests

## Method Boundaries

## Dev Runtime Behavior

## Build Runtime Behavior

## Production Runtime Behavior
