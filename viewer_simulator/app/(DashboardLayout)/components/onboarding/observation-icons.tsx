"use client"
import { Activity, Heart, ThermometerSnowflake, Stethoscope, Scale, Ruler, TreesIcon as Lungs, Droplet, Gauge } from 'lucide-react'

// Comprehensive mapping of LOINC codes to icons and display names with Dutch translations
export const observationCodeMap: { [name: string]: any } = {
    // Vital signs
    "8867-4": {
        display: "Hartslag",
        icon: <Heart className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "8310-5": {
        display: "Lichaamstemperatuur",
        icon: <ThermometerSnowflake className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "8480-6": {
        display: "Bloeddruk (Systolisch)",
        icon: <Activity className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "85354-9": {
        display: "Bloeddruk",
        icon: <Activity className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "29463-7": {
        display: "Lichaamsgewicht",
        icon: <Scale className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "8302-2": {
        display: "Lichaamslengte",
        icon: <Ruler className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "9279-1": {
        display: "Ademhalingsfrequentie",
        icon: <Lungs className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "59408-5": {
        display: "Zuurstofsaturatie",
        icon: <Gauge className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "vital-signs"
    },
    "15074-8": {
        display: "Bloedglucose",
        icon: <Droplet className="h-5 w-5" />,
        system: "http://loinc.org",
        category: "laboratory"
    }
}

// Helper function to get appropriate icon for observation type
export const getObservationIcon = (code: string) => {
    return observationCodeMap[code]?.icon || <Stethoscope className="h-5 w-5" />
}

// Get all observation codes as an array for selection components
export const getObservationCodes = () => {
    return Object.entries(observationCodeMap).map(([code, data]) => ({
        code,
        display: data.display,
        system: data.system,
        category: data.category
    }))
}

// Get observation categories with Dutch translations
export const observationCategories = [
    { display: "Vitale functies", code: "vital-signs", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
    { display: "Laboratorium", code: "laboratory", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
    { display: "Onderzoek", code: "exam", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
    { display: "Sociale geschiedenis", code: "social-history", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
    { display: "Beeldvorming", code: "imaging", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
]

// Get unit suggestions based on observation code with Dutch translations where appropriate
export const getUnitSuggestions = (code: string): string[] => {
    switch (code) {
        case "29463-7": // Body Weight
            return ["kg", "g", "lb"]
        case "8302-2": // Body Height
            return ["cm", "m", "in"]
        case "8867-4": // Heart rate
            return ["slagen/min"]
        case "85354-9": // Blood Pressure
        case "8480-6": // Blood Pressure
            return ["mmHg"]
        case "8310-5": // Body Temperature
            return ["°C", "°F"]
        case "9279-1": // Respiratory Rate
            return ["ademhalingen/min"]
        case "59408-5": // Oxygen Saturation
            return ["%"]
        case "15074-8": // Blood Glucose
            return ["mg/dL", "mmol/L"]
        default:
            return []
    }
}

// Dutch translations for interpretation codes
export const interpretationOptions = [
    { code: "N", display: "Normaal", description: "Het resultaat is normaal" },
    { code: "A", display: "Abnormaal", description: "Het resultaat is abnormaal" },
    { code: "H", display: "Hoog", description: "Het resultaat is boven het normale bereik" },
    { code: "L", display: "Laag", description: "Het resultaat is onder het normale bereik" },
    { code: "HH", display: "Kritisch Hoog", description: "Het resultaat is boven een kritiek niveau" },
    { code: "LL", display: "Kritisch Laag", description: "Het resultaat is onder een kritiek niveau" },
]

// Dutch translations for status options
export const statusOptions = [
    { code: "registered", display: "Geregistreerd" },
    { code: "preliminary", display: "Voorlopig" },
    { code: "final", display: "Definitief" },
    { code: "amended", display: "Gewijzigd" },
    { code: "corrected", display: "Gecorrigeerd" },
    { code: "cancelled", display: "Geannuleerd" },
    { code: "entered-in-error", display: "Foutief ingevoerd" },
    { code: "unknown", display: "Onbekend" },
]

// Dutch translations for common UI elements
export const uiTranslations = {
    // Form labels
    status: "Status",
    category: "Categorie",
    observationType: "Type observatie",
    dateAndTime: "Datum en tijd",
    value: "Waarde",
    unit: "Eenheid",
    referenceRange: "Referentiewaarden",
    lowValue: "Ondergrens",
    highValue: "Bovengrens",
    interpretation: "Interpretatie",
    notes: "Notities",

    // Buttons and actions
    save: "Opslaan",
    saving: "Opslaan...",
    close: "Sluiten",
    registerNew: "Nieuwe observatie registreren voor",

    // Messages
    successMessage: "Observatie succesvol geregistreerd",
    warningOutsideRange: "Waarschuwing: Waarde buiten bereik",

    // Validation errors
    requiredCode: "Type observatie is verplicht",
    requiredDateTime: "Datum en tijd zijn verplicht",
    requiredValue: "Waarde is verplicht",
    requiredRangeValues: "Zowel onder- als bovengrens zijn verplicht voor het referentiebereik",

    // Other labels
    taskId: "Taak ID",
    scpContext: "SCP Context",
    taskStatus: "Taak status",
    patientIdentifier: "Patiënt identificatie",
    focus: "Focus",
    subject: "Subject",
    performer: "Uitvoerder",
    id: "ID"
}
