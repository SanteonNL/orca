"use client"
import { InfoIcon } from "lucide-react"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"

export default function NoObservationsFound({ className }: { className?: string }) {

    return (
        <Card className={cn("w-full max-w-md mx-auto bg-background/50 border-[#1c6268] border-dashed", className)}>
            <CardHeader className="text-center pb-2">
                <div className="mx-auto w-12 h-12 rounded-full bg-muted flex items-center justify-center mb-2">
                    <InfoIcon className="h-6 w-6 text-[#1c6268]" />
                </div>
                <CardTitle className="text-2xl pt-2 first-letter:uppercase text-[#1c6268]">Geen observaties gevonden</CardTitle>
                <CardDescription className="text-center">Er zijn nog geen observaties aangemaakt binnen dit traject.</CardDescription>
            </CardHeader>

            <CardContent className="flex flex-col items-center pt-4 pb-6">
                <div className="text-sm text-muted-foreground text-center max-w-xs">
                    <p>
                        Klinische observaties omvatten vitale functies, laboratoriumresultaten en andere meetbare gezondheidsgegevens
                        die helpen de toestand van een patiënt in de loop van de tijd te volgen.
                    </p>
                </div>
            </CardContent>
        </Card>
    )
}

