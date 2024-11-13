import React from 'react'
import SwaggerUI from "swagger-ui-react"
import "swagger-ui-react/swagger-ui.css"
import {
    Alert,
    AlertDescription,
    AlertTitle,
} from "@/components/ui/alert"
import { Terminal } from 'lucide-react'


export default function FlagViewer() {
    return (
        <>
            <Alert>
                <Terminal className="h-4 w-4" />
                <AlertTitle>Flags</AlertTitle>
                <AlertDescription className='flex justify-between items-center'>
                    <span>
                        Om gebruik te maken van deze calls dient een Application Token gegenereerd te worden.
                        Deze kan middels de <code>TestService_FetchApplicationToken_IntegrationTest</code> test aangemaakt worden.
                    </span>
                </AlertDescription>
            </Alert>
            <SwaggerUI url={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/flag.json`} />
        </>
    )
}
