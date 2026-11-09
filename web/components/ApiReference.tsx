"use client";

import { ApiReferenceReact } from "@scalar/api-reference-react";
import "@scalar/api-reference-react/style.css";

export default function ApiReference() {
    return (
        <ApiReferenceReact
            configuration={{
                url: "/api/v1/openapi.json",
                hideClientButton: true,
                hideDarkModeToggle: true,
                forceDarkModeState: "light",
                withDefaultFonts: false,
                servers: [{ url: "/api/v1", description: "This server" }],
            }}
        />
    );
}
