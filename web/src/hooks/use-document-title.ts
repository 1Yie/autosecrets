import { useEffect } from "react";

const PRODUCT = "AutoSecrets";

/** Keeps document.title in sync with the current page. Passing a title
 * renders "title · AutoSecrets"; passing undefined resets to the product
 * name only. */
export function useDocumentTitle(title?: string) {
	useEffect(() => {
		document.title = title ? `${title} · ${PRODUCT}` : PRODUCT;
	}, [title]);
}
