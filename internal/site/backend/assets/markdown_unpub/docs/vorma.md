---
title: Docs 2
description: Documentation for the Vorma framework
order: 3
---

# Vorma Documentation

## Starting a New Vorma Project

`npm create vorma@latest`

## Supported Frontend UI Libraries

- React
- Preact
- Solid

## Wave Config

Vorma's lower-level build tool is called Wave. Wave handles things like watching
for dev-time file changes, running build hooks, integrating with the Vite CLI,
and handling project structure customizations.

You can configure Wave via a JSON file (your "`WAVE_CONFIG`"), which typically
lives at `./backend/wave.config.json`.

To learn more, visit the
[Wave configuration docs](/docs/advanced/configuring-wave).

## Public Static Assets

- Your public static assets directory typically lives at `./frontend/assets`.
  This is configurable via the `Core.StaticAssetDirs.Public` property in your
  `WAVE_CONFIG`.
- Any files you put in this directory will be hashed and made publicly available
  to visitors of your app. Because assets are hashed, it's safe to serve these
  files with immutable cache headers.
- Files are served from the public path prefix base, which is set via the
  `Core.PublicPathPrefix` property in your `WAVE_CONFIG`. This is usually set to
  `/`, but `/public/` is also common.
- If you are programmatically adding files to this directory that have already
  been hashed (most likely as a result of some side build process), you can tell
  Wave to skip hashing by putting them into a `/prehashed/` child directory
  inside your public static assets directory (_e.g._,
  `./frontend/assets/prehashed/`).

#### Accessing At Build-Time

##### On The Frontend

##### On The Backend

#### Accessing At Runtime

##### On The Frontend

##### On The Backend

### Private Assets

#### HTML Entry Template

##### Injecting Things Into Your Build Template

## Backend Directory

### Core Router

#### Serve Static Middleware

#### Health Check Endpoint

#### Loaders Handler

#### Actions Handler

### API Actions

#### Actions Router

##### Pattern Matching

#### Queries / Mutations

#### Validation

### Nested UI Loaders

#### Loaders Router

##### Pattern Matching

## Control Directory

### Cmd

#### Build

##### Passing Go Values To Frontend

##### Passing Go Types To Frontend

#### Serve

##### Vorma Init

##### Graceful Shutdown

### Dist

### Vorma Instantiation and Config

### Wave JSON Config

## Frontend Directory

### Writing a Route

#### Layout Routes

#### Index Routes

#### Child Routes

#### Route Props

### Registering a Route

### Reading Server Loader Data

`useLoaderData(props)`

### Reading Server Loader Data From Another Route

`usePatternLoaderData("/some-route")`

###
