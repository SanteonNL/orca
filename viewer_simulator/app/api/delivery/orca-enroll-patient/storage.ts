import { Bundle } from "fhir/r4";

let notificationBundles: Bundle[] = [];

export function storeNotificationBundle(bundle: Bundle) {
    notificationBundles.push(bundle);
}

export async function getNotificationBundles(): Promise<Bundle[]> {
    return notificationBundles;
}
