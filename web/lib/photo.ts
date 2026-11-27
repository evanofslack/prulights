import type { Sighting, SightingPhoto } from "@/interfaces/api";
import { api } from "./api";

// Photos are shrunk in the browser before they're sent: a phone's picture is
// several MB, and drawing it to a canvas turns HEIC (Safari decodes it) into
// a jpeg the server can read, upright. The server re-encodes it again, which
// is what drops the phone's location.

const LONG_SIDE = 2000;

export async function toJpeg(file: File): Promise<Blob> {
    let bmp: ImageBitmap;
    try {
        bmp = await createImageBitmap(file, { imageOrientation: "from-image" });
    } catch {
        throw new Error("Couldn't open that picture. Try a JPEG or PNG.");
    }
    const scale = Math.min(1, LONG_SIDE / Math.max(bmp.width, bmp.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.max(1, Math.round(bmp.width * scale));
    canvas.height = Math.max(1, Math.round(bmp.height * scale));
    canvas.getContext("2d")!.drawImage(bmp, 0, 0, canvas.width, canvas.height);
    bmp.close();
    return new Promise((resolve, reject) =>
        canvas.toBlob(
            (b) => (b ? resolve(b) : reject(new Error("Couldn't read that picture."))),
            "image/jpeg",
            0.85,
        ),
    );
}

export async function uploadPhoto(date: string, file: File): Promise<Sighting> {
    const body = await toJpeg(file);
    return api<Sighting>(`/me/sightings/${date}/photo`, {
        method: "PUT",
        body,
        headers: { "Content-Type": "image/jpeg" },
    });
}

export const photoURL = (p: SightingPhoto, size: "full" | "thumb") => `/api/v1/photos/${p.sha256}/${size}.jpg`;
