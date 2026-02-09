/******************************************************************************

This file is a condensed, comment-stripped, and prefixed-type version of
npm:history's index.d.ts file as of v5.3.0. npm:history is licensed under the
MIT license. It is used under the hood by vorma/client.

The npm:history repository is located at: https://github.com/remix-run/history

It's only purpose is to re-export a minimal version of the types needed by
vorma/client's "getHistoryInstance" function, which simply returns an
instance of npm:history's BrowserHistory.

Original license:

MIT License

Copyright (c) React Training 2016-2020
Copyright (c) Remix Software 2020-2021

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

******************************************************************************/

declare enum historyAction {
	Pop = "POP",
	Push = "PUSH",
	Replace = "REPLACE",
}
declare type historyPathname = string;
declare type historySearch = string;
declare type historyHash = string;
declare type historyKey = string;
interface historyPath {
	pathname: historyPathname;
	search: historySearch;
	hash: historyHash;
}
export interface historyLocation extends historyPath {
	state: unknown;
	key: historyKey;
}
interface historyUpdate {
	action: historyAction;
	location: historyLocation;
}
export interface historyListener {
	(update: historyUpdate): void;
}
interface historyTransition extends historyUpdate {
	retry(): void;
}
interface historyBlocker {
	(tx: historyTransition): void;
}
declare type historyTo = string | Partial<historyPath>;
export interface historyInstance {
	readonly action: historyAction;
	readonly location: historyLocation;
	createHref(to: historyTo): string;
	push(to: historyTo, state?: any): void;
	replace(to: historyTo, state?: any): void;
	go(delta: number): void;
	back(): void;
	forward(): void;
	listen(listener: historyListener): () => void;
	block(blocker: historyBlocker): () => void;
}
