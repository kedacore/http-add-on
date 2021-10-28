import axios from 'axios';

export interface Response {
    status: number
    elapsedMS: number
}
// httpRequest makes a GET request to the given url
// and returns the number of milliseconds from sending
// the request until the response comes back
export async function httpRequest(url: string): Promise<Response> {
    const start = Date.now();
    const res = await axios.get(url);
    const elapsed = Date.now() - start;
    return Promise.resolve({
        status: res.status,
        elapsedMS: elapsed
    })
}
