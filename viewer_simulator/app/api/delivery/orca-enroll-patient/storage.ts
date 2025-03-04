import { Bundle } from 'fhir/r4';

let notificationBundles: Bundle[] = [];

export function storeNotificationBundle(bundle: Bundle) {
    notificationBundles.push(bundle);
}

export function getNotificationBundles(): Bundle[] {
    return notificationBundles;
}