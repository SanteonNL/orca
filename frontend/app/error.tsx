'use client' // Error boundaries must be Client Components

import { Button } from '@/components/ui/button'
import { RefreshCw, AlertTriangle } from 'lucide-react'
import { getSupportContactLink } from './actions'
import { useEffect, useState } from 'react'

export default function Error({
    error,
    reset,
}: {
    error: Error & { digest?: string }
    reset: () => void
}) {
    const [supportContactLink, setSupportContactLink] = useState<string>()

    useEffect(() => {

        const fetchSupportContactLink = async () => {
            const link = await getSupportContactLink()
            setSupportContactLink(link)
        }

        if (!supportContactLink) fetchSupportContactLink()
    }, [])

    return (
        <div className="min-h-screen flex flex-col justify-center items-center bg-gray-50">
            <div className="max-w-3xl bg-white shadow-md rounded-xl p-8 space-y-6">
                <div className="flex items-center space-x-3">
                    <AlertTriangle className="text-primary" size={30} />
                    <h2 className="text-2xl font-semibold text-gray-800">
                        Oeps, er ging iets mis!
                    </h2>
                </div>

                <p className="text-gray-600">
                    {error.message || 'Er is een onverwachte fout opgetreden. Probeer het later opnieuw.'}
                </p>

                <h3 className="text-lg font-semibold text-gray-800">
                    Tijdelijke fout?
                </h3>
                <p className="text-gray-600">
                    Tijdelijke fouten kunnen soms opgelost worden door ze opnieuw te proberen. Gebruik de onderstaande knop om het opnieuw te proberen.
                </p>
                <Button onClick={reset}>
                    <RefreshCw /> Opnieuw proberen
                </Button>

                <h3 className="text-xl font-semibold text-gray-800">
                    Aanhoudende fout?
                </h3>
                <p className="text-gray-600">
                    Indien de fout aanhoudt, neem dan contact op met support. Vermeld hierbij code
                    <code className='bg-gray-100 text-gray-800 rounded px-2 py-1 ml-1'>
                        {error.digest || '000000000'}
                    </code> en tijdstip
                    <code className='bg-gray-100 text-gray-800 rounded px-2 py-1 ml-1'>
                        {new Date().toISOString() /* Print in UTC to avoid timezone issues */}
                    </code>
                    . Deze informatie helpt bij het gericht zoeken naar de oorzaak.
                </p>
                {supportContactLink && (
                    <p className="text-sm text-gray-500">
                        Neem <a href={supportContactLink} className="text-blue-600 hover:underline">contact</a> op.
                    </p>
                )}
            </div>
        </div>
    )
}
