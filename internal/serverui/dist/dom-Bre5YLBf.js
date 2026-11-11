// src/dom.ts
function element(tag, text, className) {
	const node = document.createElement(tag);
	if (text !== void 0) node.textContent = text;
	if (className) node.className = className;
	return node;
}
function required(selector, root = document) {
	const node = root.querySelector(selector);
	if (!node) throw new Error(`Missing UI element: ${selector}`);
	return node;
}
function errorMessage(error) {
	return error instanceof Error ? error.message : "Request failed.";
}
export { errorMessage as n, required as r, element as t };
