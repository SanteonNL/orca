import React from 'react'
import SwaggerUI from "swagger-ui-react"
import "swagger-ui-react/swagger-ui.css"
import {
    Alert,
    AlertDescription,
    AlertTitle,
} from "@/components/ui/alert"
import { FlagIcon, Terminal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import Link from 'next/link'


export default function BgzViewer() {
    return (
        <>
            <Alert>
                <Terminal className="h-4 w-4" />
                <AlertTitle>Flags</AlertTitle>
                <AlertDescription className='flex justify-between items-center'>
                    Onderstaande calls maken gebruik van een HCP Token. Om een Flag aan te maken, dien je gebruik te maken
                    van een Application token.
                    <Button asChild>
                        <Link href={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/viewer/flag`}>
                            <FlagIcon className='mr-2' /> Flags
                        </Link>
                    </Button>
                </AlertDescription>
            </Alert>
            <SwaggerUI url={`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/bgz.json`} />
        </>
    )
}
