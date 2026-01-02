export function logInfo(message?: any, ...optionalParams: Array<any>) {
	console.log("Vorma:", message, ...optionalParams);
}

export function logError(message?: any, ...optionalParams: Array<any>) {
	console.error("Vorma:", message, ...optionalParams);
}
