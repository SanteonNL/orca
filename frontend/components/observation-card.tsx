"use client"
import { useState } from "react"
import { formatWithOptions } from "date-fns/fp"
import { AlertCircle, Calendar, ChevronDown, Clock, FileText, Info, Stethoscope, User } from "lucide-react"
import { motion, AnimatePresence } from "framer-motion"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import type { Observation } from "fhir/r4"
import { getObservationIcon } from "./observation-icons"
import { identifierToString } from "@/lib/fhirUtils"
import { nl } from "date-fns/locale"

// Animation variants
const contentVariants = {
    hidden: {
        height: 0,
        opacity: 0,
        transition: {
            height: { duration: 0.3, ease: "easeInOut" },
            opacity: { duration: 0.2 },
        },
    },
    visible: {
        height: "auto",
        opacity: 1,
        transition: {
            height: { duration: 0.3, ease: "easeInOut" },
            opacity: { duration: 0.2, delay: 0.1 },
        },
    },
}

// Helper function to get color based on status
const getStatusColor = (status: Observation["status"]) => {
    const statusColorMap: Record<string, string> = {
        final: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300",
        preliminary: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300",
        amended: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300",
        corrected: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300",
        cancelled: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300",
        "entered-in-error": "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300",
        unknown: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300",
        registered: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300",
    }

    return statusColorMap[status] || statusColorMap.unknown
}

// Helper function to check if value is abnormal
const isAbnormal = (observation: Observation) => {
    if (!observation.interpretation?.length) return false

    const abnormalCodes = ["A", "AA", "HH", "LL", "H", "L"]
    return observation.interpretation.some((i) =>
        i.coding?.some((code) => code.code && abnormalCodes.includes(code.code)),
    )
}

export default function ObservationCard({
    observation,
    showDetails = false,
}: {
    observation: Observation
    showDetails?: boolean
}) {
    const [expanded, setExpanded] = useState(showDetails)

    // Extract main display values
    const observationName = observation.code.text || observation.code.coding?.[0]?.display || "Onbekend"
    const observationCode = observation.code.coding?.[0]?.code || "Onbekend"
    const getObservationValue = (observation: Observation) => {
        if (observation.valueQuantity) return `${observation.valueQuantity.value} ${observation.valueQuantity.unit}`
        if (observation.valueString) return observation.valueString
        if (observation.valueCodeableConcept?.text) return observation.valueCodeableConcept.text
        if (observation.valueBoolean !== undefined) return observation.valueBoolean ? "Ja" : "Nee"
        if (observation.valueInteger !== undefined) return observation.valueInteger.toString()
        return "Unsupported value type"
    }

    const effectiveDate = observation.effectiveDateTime
        ? formatWithOptions({ locale: nl }, "d MMM yyyy", new Date(observation.effectiveDateTime))
        : "Onbekende datum"

    const effectiveTime = observation.effectiveDateTime ? formatWithOptions({ locale: nl }, "HH:mm", new Date(observation.effectiveDateTime)) : ""

    const abnormal = isAbnormal(observation)

    return (
        <Card className={cn("w-full transition-all duration-200", abnormal && "border-red-300 dark:border-red-800")}>
            <CardHeader className="pb-2">
                <div className="flex justify-between items-start">
                    <div className="flex items-center gap-2">
                        {getObservationIcon(observationCode)}
                        <CardTitle className="text-lg">{observationName}</CardTitle>
                    </div>
                    <Badge className={getStatusColor(observation.status)}>
                        {observation.status.charAt(0).toUpperCase() + observation.status.slice(1).replace(/-/g, " ")}
                    </Badge>
                </div>
                <CardDescription className="flex items-center gap-1 mt-1">
                    <Calendar className="h-3.5 w-3.5" />
                    <span>{effectiveDate}</span>
                    {effectiveTime && (
                        <>
                            <Clock className="h-3.5 w-3.5 ml-2" />
                            <span>{effectiveTime}</span>
                        </>
                    )}
                </CardDescription>
            </CardHeader>

            <CardContent>
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                        <span className={cn("text-2xl font-semibold", abnormal && "text-red-600 dark:text-red-400")}>
                            {getObservationValue(observation)}
                        </span>
                        {abnormal && (
                            <TooltipProvider>
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <AlertCircle className="h-5 w-5 text-red-500" />
                                    </TooltipTrigger>
                                    <TooltipContent>
                                        <p>Abnormale waarde</p>
                                    </TooltipContent>
                                </Tooltip>
                            </TooltipProvider>
                        )}
                    </div>

                    <motion.div whileTap={{ scale: 0.95 }}>
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setExpanded(!expanded)}
                            className="p-1 h-8"
                            aria-expanded={expanded}
                            aria-label={expanded ? "Collapse details" : "Expand details"}
                        >
                            <motion.div animate={{ rotate: expanded ? 180 : 0 }} transition={{ duration: 0.3 }}>
                                <ChevronDown className="h-5 w-5" />
                            </motion.div>
                        </Button>
                    </motion.div>
                </div>

                {observation.referenceRange && observation.referenceRange.length > 0 && (
                    <div className="mt-2 text-sm text-muted-foreground">
                        <span>Reference: </span>
                        {observation.referenceRange.map((range, i) => (
                            <span key={i}>
                                {range.text || (
                                    <>
                                        {range.low && `${range.low.value} ${range.low.unit}`}
                                        {range.low && range.high && " - "}
                                        {range.high && `${range.high.value} ${range.high.unit}`}
                                    </>
                                )}
                            </span>
                        ))}
                    </div>
                )}

                <AnimatePresence initial={false}>
                    {expanded && (
                        <motion.div
                            className="overflow-hidden"
                            variants={contentVariants}
                            initial="hidden"
                            animate="visible"
                            exit="hidden"
                        >
                            <div className="mt-4 space-y-3 text-sm border-t pt-3">
                                {observation.category && (
                                    <motion.div
                                        className="flex items-start gap-2"
                                        initial={{ opacity: 0, y: 10 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        transition={{ delay: 0.1 }}
                                    >
                                        <FileText className="h-4 w-4 mt-0.5 text-muted-foreground" />
                                        <div>
                                            <p className="font-medium">Categorie</p>
                                            <div className="flex flex-wrap gap-1 mt-1">
                                                {observation.category.map((cat, i) => (
                                                    <Badge key={i} variant="outline">
                                                        {cat.text || cat.coding?.[0]?.display || "Unknown"}
                                                    </Badge>
                                                ))}
                                            </div>
                                        </div>
                                    </motion.div>
                                )}

                                <motion.div
                                    className="flex items-start gap-2"
                                    initial={{ opacity: 0, y: 10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    transition={{ delay: 0.2 }}
                                >
                                    <User className="h-4 w-4 mt-0.5 text-muted-foreground" />
                                    <div>
                                        <p className="font-medium">Subject</p>
                                        <p>
                                            {observation.subject?.display ||
                                                identifierToString(observation.subject?.identifier) ||
                                                observation.subject?.reference ||
                                                "Onbekend"}
                                        </p>
                                    </div>
                                </motion.div>

                                {observation.performer && observation.performer.length > 0 && (
                                    <motion.div
                                        className="flex items-start gap-2"
                                        initial={{ opacity: 0, y: 10 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        transition={{ delay: 0.3 }}
                                    >
                                        <Stethoscope className="h-4 w-4 mt-0.5 text-muted-foreground" />
                                        <div>
                                            <p className="font-medium">Uitvoerder</p>
                                            <p>{observation.performer[0].display || observation.performer[0].reference}</p>
                                        </div>
                                    </motion.div>
                                )}

                                {observation.note && observation.note.length > 0 && (
                                    <motion.div
                                        className="flex items-start gap-2"
                                        initial={{ opacity: 0, y: 10 }}
                                        animate={{ opacity: 1, y: 0 }}
                                        transition={{ delay: 0.4 }}
                                    >
                                        <Info className="h-4 w-4 mt-0.5 text-muted-foreground" />
                                        <div>
                                            <p className="font-medium">Notities</p>
                                            {observation.note.map((note, i) => (
                                                <p key={i} className="mt-1">
                                                    {note.text}
                                                </p>
                                            ))}
                                        </div>
                                    </motion.div>
                                )}
                                <motion.div
                                    className="flex items-start gap-2"
                                    initial={{ opacity: 0, y: 10 }}
                                    animate={{ opacity: 1, y: 0 }}
                                    transition={{ delay: 0.5 }}
                                >
                                    <div className="pt-0 text-xs text-muted-foreground">ID: {observation.id}</div>
                                </motion.div>

                            </div>
                        </motion.div>
                    )}
                </AnimatePresence>
            </CardContent>
        </Card>
    )
}

