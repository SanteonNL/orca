"use client"
import React, { useState, useEffect } from "react"
import { IconPencilPlus } from "@tabler/icons-react"
import type { Observation, Task, Reference, Identifier } from "fhir/r4"
import Dialog from "@mui/material/Dialog"
import AppBar from "@mui/material/AppBar"
import Toolbar from "@mui/material/Toolbar"
import Typography from "@mui/material/Typography"
import CloseIcon from "@mui/icons-material/Close"
import Slide from "@mui/material/Slide"
import type { TransitionProps } from "@mui/material/transitions"
import {
  Box,
  IconButton,
  TextField,
  MenuItem,
  Select,
  FormControl,
  InputLabel,
  Button,
  Grid,
  FormHelperText,
  Snackbar,
  Alert,
  CircularProgress,
  Chip,
  Portal,
} from "@mui/material"
import { DateTimePicker, LocalizationProvider } from "@mui/x-date-pickers"
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs"
import dayjs from "dayjs"
import "dayjs/locale/nl"
import { getAbsoluteCarePlanReference } from "@/utils/fhirUtils"

const Transition = React.forwardRef(function Transition(
  props: TransitionProps & {
    children: React.ReactElement
  },
  ref: React.Ref<unknown>,
) {
  return <Slide direction="up" ref={ref} {...props} />
})

// Common observation categories in FHIR
const observationCategories = [
  { display: "Vital Signs", code: "vital-signs", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
  { display: "Laboratory", code: "laboratory", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
  { display: "Exam", code: "exam", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
  {
    display: "Social History", code: "social-history", system: "http://terminology.hl7.org/CodeSystem/observation-category",
  },
  { display: "Imaging", code: "imaging", system: "http://terminology.hl7.org/CodeSystem/observation-category" },
]

// Common observation codes (LOINC)
const observationCodes = [
  { display: "Body Weight", code: "29463-7", system: "http://loinc.org" },
  { display: "Body Height", code: "8302-2", system: "http://loinc.org" },
  { display: "Heart rate", code: "8867-4", system: "http://loinc.org" },
  { display: "Blood Pressure", code: "85354-9", system: "http://loinc.org" },
  { display: "Body Temperature", code: "8310-5", system: "http://loinc.org" },
  { display: "Respiratory Rate", code: "9279-1", system: "http://loinc.org" },
  { display: "Oxygen Saturation", code: "59408-5", system: "http://loinc.org" },
  { display: "Blood Glucose", code: "15074-8", system: "http://loinc.org" },
]

// Observation status options
const statusOptions: Observation["status"][] = [
  "registered",
  "preliminary",
  "final",
  "amended",
  "corrected",
  "cancelled",
  "entered-in-error",
  "unknown",
]

const getUnitSuggestions = (code: string): string[] => {
  // Return appropriate units based on the selected observation code
  switch (code) {
    case "29463-7": // Body Weight
      return ["kg", "g", "lb"]
    case "8302-2": // Body Height
      return ["cm", "m", "in"]
    case "8867-4": // Heart rate
      return ["beats/min"]
    case "85354-9": // Blood Pressure
      return ["mmHg"]
    case "8310-5": // Body Temperature
      return ["°C", "°F"]
    case "9279-1": // Respiratory Rate
      return ["breaths/min"]
    case "59408-5": // Oxygen Saturation
      return ["%"]
    case "15074-8": // Blood Glucose
      return ["mg/dL", "mmol/L"]
    default:
      return []
  }
}

/**
 * Creates a reference if an identifier and/or absolute reference is present for cross-system compatibility.
 * If only a relative reference is present, returns undefined.
 */
function createExternalReference(reference: Reference | undefined): Reference | undefined {
  if (!reference) return undefined

  const externalRef = { ...reference }

  // If there is an absolute reference, return a basic reference
  if (externalRef.reference && !externalRef.reference.startsWith("http")) {
    externalRef.reference = undefined
  }

  // If the reference only contained a relative ref and no identifier, return undefined
  if (!externalRef.reference && !externalRef.identifier?.value) {
    return undefined
  }

  return externalRef
}

/**
 * Extracts a display name from a reference or identifier
 */
function getDisplayFromReference(reference: Reference | undefined): string {
  if (!reference) return "Unknown"

  if (reference.display) {
    return reference.display
  }

  if (reference.identifier?.value) {
    return `${reference.type || "Resource"} (${reference.identifier.system?.split("/").pop() || "ID"}: ${reference.identifier.value})`
  }

  if (reference.reference) {
    return reference.reference
  }

  return "Unknown"
}

export default function CreateTaskObservation({ task, taskFullUrl }: { task: Task, taskFullUrl: string }) {
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [showSuccess, setShowSuccess] = useState(false)
  const [patientDisplay, setPatientDisplay] = useState<string>("")
  const [loading, setLoading] = useState(false)
  const [scpContextIdentifier, setScpContextIdentifier] = useState<Identifier | undefined>()

  // Form state
  const [status, setStatus] = useState<string>("preliminary")
  const [category, setCategory] = useState<string>("vital-signs")
  const [code, setCode] = useState<string>("")
  const [effectiveDateTime, setEffectiveDateTime] = useState<dayjs.Dayjs | null>(dayjs())
  const [valueQuantity, setValueQuantity] = useState<string>("")
  const [valueUnit, setValueUnit] = useState<string>("")
  const [note, setNote] = useState<string>("")

  // Form validation
  const [errors, setErrors] = useState<Record<string, string>>({})

  // Extract patient information and task identifier when component mounts
  useEffect(() => {
    if (task) {
      // Set patient display name
      if (task.for) {
        setPatientDisplay(getDisplayFromReference(task.for))
      }
    }
  }, [task])

  useEffect(() => {
    setScpContextIdentifier(getAbsoluteCarePlanReference(task, taskFullUrl))
  }, [task, taskFullUrl])

  const handleClickOpen = () => {
    setOpen(true)

    // Pre-populate form based on task if needed
    if (task.status === "completed") {
      setStatus("final")
    } else if (task.status === "in-progress") {
      setStatus("preliminary")
    } else {
      setStatus("registered")
    }
  }

  const handleClose = () => {
    setOpen(false)
    resetForm()
  }

  const resetForm = () => {
    setStatus("preliminary")
    setCategory("vital-signs")
    setCode("")
    setEffectiveDateTime(dayjs())
    setValueQuantity("")
    setValueUnit("")
    setNote("")
    setErrors({})
  }

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {}

    if (!code) newErrors.code = "Observation code is required"
    if (!effectiveDateTime) newErrors.effectiveDateTime = "Date and time are required"
    if (!valueQuantity) newErrors.valueQuantity = "Value is required"

    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const handleSave = async () => {
    if (!validateForm()) return

    if (!scpContextIdentifier) throw Error("SCP Context Identifier is required - it's undefined")

    setSaving(true)

    try {
      // Create the FHIR Observation resource
      const selectedCategory = observationCategories.find((c) => c.code === category)
      const selectedCode = observationCodes.find((c) => c.code === code)

      const observation: Observation = {
        resourceType: "Observation",
        status: status as Observation["status"],
        identifier: [scpContextIdentifier || { system: "urn:uuid", value: "uuid" }],
        category: [
          {
            coding: [
              {
                system: selectedCategory?.system,
                code: selectedCategory?.code,
                display: selectedCategory?.display,
              },
            ],
          },
        ],
        code: {
          coding: [
            {
              system: selectedCode?.system,
              code: selectedCode?.code,
              display: selectedCode?.display,
            },
          ],
        },
        // Use subject with identifier when possible for cross-system compatibility
        subject: createExternalReference(task.for),
        effectiveDateTime: effectiveDateTime?.toISOString(),
        valueQuantity: {
          value: Number.parseFloat(valueQuantity),
          unit: valueUnit,
          system: "http://unitsofmeasure.org",
          code: valueUnit,
        },
      }

      // Add note if provided
      if (note) {
        observation.note = [
          {
            text: note,
          },
        ]
      }

      // Link to task using basedOn with identifier when possible
      if (taskFullUrl) {
        observation.basedOn = [
          {
            type: "Task",
            reference: taskFullUrl,
            display: taskFullUrl,
          },
        ]
      } else if (task.id) {
        observation.basedOn = [
          {
            reference: `Task/${task.id}`,
            type: "Task",
          },
        ]
      }

      // Add focus reference if task has one, using identifier when possible
      if (task.focus) {
        const ref = createExternalReference(task.focus)
        if (ref) observation.focus = [ref]
      }

      // Add partOf references if task is part of other tasks
      if (task.partOf && task.partOf.length > 0) {
        observation.partOf = task.partOf
          .map((ref) => createExternalReference(ref))
          .filter((ref) => ref !== undefined) as Reference[]
      }

      // Add context from task's encounter or episode if available
      if (task.encounter) {
        observation.encounter = createExternalReference(task.encounter)
      }

      const resp = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH || ""}/api/fhir/Observation`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(observation),
      })

      if (!resp.ok) {
        const errorMsg = JSON.stringify(await resp.json(), undefined, 2)
        throw new Error(`Failed to save observation: [${resp.status}] ${errorMsg || resp.statusText || "No server message"}`)
      }

      setShowSuccess(true)
      handleClose()
    } catch (error) {
      console.error("Error saving observation:", error)
      if (error instanceof Error) {
        setErrors({ submit: error.message || "Failed to save observation. Please try again." })
      } else {
        setErrors({ submit: "Failed to save observation. Please try again." })
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale="nl">
      <IconButton edge="start" color="inherit" onClick={handleClickOpen} aria-label="register observation">
        <IconPencilPlus />
      </IconButton>

      <Dialog open={open} fullScreen TransitionComponent={Transition}>
        <AppBar sx={{ position: "fixed", backgroundColor: "#121212" }}>
          <Toolbar>
            <IconButton edge="start" color="inherit" onClick={handleClose} aria-label="close" disabled={saving}>
              <CloseIcon />
            </IconButton>
            <Typography sx={{ ml: 2, flex: 1 }} variant="h6" component="div">
              Register New Observation for {patientDisplay}
            </Typography>
            <Button color="inherit" onClick={handleSave} disabled={saving}>
              {saving ? "Saving..." : "Save"}
            </Button>
          </Toolbar>
        </AppBar>

        <Box sx={{ mt: "80px", p: 3 }}>
          {loading ? (
            <Box sx={{ display: "flex", justifyContent: "center", alignItems: "center", height: "50vh" }}>
              <CircularProgress />
            </Box>
          ) : (
            <Grid container spacing={3}>
              {/* Task Information */}
              <Grid item xs={12}>
                <Box sx={{ p: 2, bgcolor: "background.paper", borderRadius: 1, mb: 2 }}>
                  <Typography variant="subtitle2" color="text.secondary">
                    Task ID: {task.id}
                    {scpContextIdentifier && (
                      <Chip
                        color="primary"
                        size="small"
                        label={`SCP Context: ${scpContextIdentifier.system?.split("/").pop()}: ${scpContextIdentifier.value}`}
                        sx={{ ml: 1 }}
                      />
                    )}
                  </Typography>
                  {task.focus && (
                    <Typography variant="subtitle2" color="text.secondary">
                      Focus: {getDisplayFromReference(task.focus)}
                    </Typography>
                  )}
                  <Typography variant="subtitle2" color="text.secondary">
                    Task Status: {task.status}
                  </Typography>
                  {task.for?.identifier && (
                    <Typography variant="subtitle2" color="text.secondary">
                      Patient Identifier: {task.for.identifier.system?.split("/").pop()}: {task.for.identifier.value}
                    </Typography>
                  )}
                </Box>
              </Grid>

              {/* Status */}
              <Grid item xs={12} md={6}>
                <FormControl fullWidth error={!!errors.status}>
                  <InputLabel id="status-label">Status</InputLabel>
                  <Select
                    labelId="status-label"
                    value={status}
                    label="Status"
                    onChange={(e) => setStatus(e.target.value)}
                  >
                    {statusOptions.map((option) => (
                      <MenuItem key={option} value={option}>
                        {option.charAt(0).toUpperCase() + option.slice(1).replace(/-/g, " ")}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.status && <FormHelperText>{errors.status}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Category */}
              <Grid item xs={12} md={6}>
                <FormControl fullWidth error={!!errors.category}>
                  <InputLabel id="category-label">Category</InputLabel>
                  <Select
                    labelId="category-label"
                    value={category}
                    label="Category"
                    onChange={(e) => setCategory(e.target.value)}
                  >
                    {observationCategories.map((category) => (
                      <MenuItem key={category.code} value={category.code}>
                        {category.display}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.category && <FormHelperText>{errors.category}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Observation Code */}
              <Grid item xs={12}>
                <FormControl fullWidth error={!!errors.code}>
                  <InputLabel id="code-label">Observation Type</InputLabel>
                  <Select
                    labelId="code-label"
                    value={code}
                    label="Observation Type"
                    onChange={(e) => {
                      setCode(e.target.value)
                      // Reset unit when code changes
                      setValueUnit("")
                    }}
                  >
                    {observationCodes.map((code) => (
                      <MenuItem key={code.code} value={code.code}>
                        {code.display}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.code && <FormHelperText>{errors.code}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Effective Date Time */}
              <Grid item xs={12}>
                <DateTimePicker
                  maxDate={dayjs()}
                  label="Date and Time"
                  value={effectiveDateTime}
                  onChange={(newValue) => setEffectiveDateTime(newValue)}
                  slotProps={{
                    textField: {
                      fullWidth: true,
                      error: !!errors.effectiveDateTime,
                      helperText: errors.effectiveDateTime,
                    },
                  }}
                />
              </Grid>
              {/* Value */}
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  label="Value"
                  type="number"
                  value={valueQuantity}
                  onChange={(e) => setValueQuantity(e.target.value)}
                  error={!!errors.valueQuantity}
                  helperText={errors.valueQuantity}
                />
              </Grid>

              {/* Unit */}
              <Grid item xs={12} md={6}>
                <FormControl fullWidth error={!!errors.valueUnit}>
                  <InputLabel id="unit-label">Unit</InputLabel>
                  <Select
                    labelId="unit-label"
                    value={valueUnit}
                    label="Unit"
                    onChange={(e) => setValueUnit(e.target.value)}
                  >
                    {getUnitSuggestions(code).map((unit) => (
                      <MenuItem key={unit} value={unit}>
                        {unit}
                      </MenuItem>
                    ))}
                  </Select>
                  {errors.valueUnit && <FormHelperText>{errors.valueUnit}</FormHelperText>}
                </FormControl>
              </Grid>

              {/* Notes */}
              <Grid item xs={12}>
                <TextField
                  fullWidth
                  label="Notes"
                  multiline
                  rows={4}
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                />
              </Grid>

              {/* General error message */}
              {errors.submit && (
                <Grid item xs={12}>
                  <Alert severity="error">{errors.submit}</Alert>
                </Grid>
              )}
            </Grid>
          )}
        </Box>
      </Dialog>

      <Portal>

        <Snackbar
          open={showSuccess}
          autoHideDuration={4000}
          onClose={() => setShowSuccess(false)}
          anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        >
          <Alert onClose={() => setShowSuccess(false)} severity="success">
            Observation successfully registered
          </Alert>
        </Snackbar>
      </Portal>
    </LocalizationProvider>
  )
}