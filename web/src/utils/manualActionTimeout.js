// Manual project jobs are tracked on the server and survive navigation away
// from the page. Large image pulls should not inherit a five minute deadline.
export const DEFAULT_MANUAL_ACTION_TIMEOUT_SECONDS = 0;
