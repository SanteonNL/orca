import { Bundle } from 'fhir/r4';

let notificationBundles: Bundle[] = [];

export function storeNotificationBundle(bundle: Bundle) {
    notificationBundles.push(bundle);
}

export async function getNotificationBundles(): Promise<Bundle[]> {

    //fetch all data when developing on the dev server, so we don't have to constantly create new resources in the hospital simulator
    if (!notificationBundles?.length && process.env.NODE_ENV === 'development') {
        console.info('[NODE_ENV === "development"] No notification bundles found - Fetching resources from FHIR server');
        await fetchAllResources()
    }

    return notificationBundles;
}

async function fetchAllResources() {

    const everythingResponses = await fetch(`${process.env.FHIR_BASE_URL}/Patient/$everything?_count=500`, {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN || 'unconfigured'}`
        }
    })

    if (everythingResponses.ok) {
        notificationBundles = [await everythingResponses.json()]
        console.log(`Fetched all resources from FHIR server - found ${notificationBundles[0].total} resource(s)`);
    } else {
        console.error(`Failed to fetch resources from FHIR server for any patients`);
    }

}