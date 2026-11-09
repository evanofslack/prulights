"use client";

import { useEffect, useRef, useState } from "react";
import Script from "next/script";

// The api reference: stock Swagger UI on its own page, reading the spec the go
// server serves. Swagger's own prebuilt bundle is copied into public/swagger
// at build time (npm run swagger) and loaded as is: bundling swagger-ui
// through next breaks its OpenAPI 3.1 support.

declare global {
    interface Window {
        SwaggerUIBundle?: (options: Record<string, unknown>) => unknown;
    }
}

export default function DocsPage() {
    const el = useRef<HTMLDivElement>(null);
    const [ready, setReady] = useState(false);
    useEffect(() => {
        if (!ready || !el.current || !window.SwaggerUIBundle) return;
        window.SwaggerUIBundle({
            url: "/api/v1/openapi.json",
            domNode: el.current,
            docExpansion: "list",
            defaultModelsExpandDepth: 0,
            tryItOutEnabled: true,
            deepLinking: true,
        });
    }, [ready]);
    return (
        <div className="min-h-dvh bg-white" style={{ colorScheme: "light" }}>
            <link rel="stylesheet" href="/swagger/swagger-ui.css" />
            <Script
                src="/swagger/swagger-ui-bundle.js"
                strategy="afterInteractive"
                onReady={() => setReady(true)}
            />
            <div ref={el}>
                <p className="p-6 text-[#3b4151]">Loading the API reference…</p>
            </div>
        </div>
    );
}
