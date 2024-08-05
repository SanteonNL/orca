"use client"
import * as React from "react"
import { Check, ChevronsUpDown } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover"
import { useEffect } from "react"

interface Record {
    value: string,
    label: string,
}
interface Props {
    records?: Record[]
    selectedValue?: string
    onChange(value: string): void
    className?: string
    disabled?: boolean
}

const Combobox: React.FC<Props> = ({ records, selectedValue, onChange, className, disabled }) => {
    const [open, setOpen] = React.useState(false)
    const [value, setValue] = React.useState(selectedValue || "")

    useEffect(() => {
        if (onChange) onChange(value)
    }, [value])

    // Effect to update internal value when selectedValue changes
    useEffect(() => {
        setValue(selectedValue || "")
    }, [selectedValue])

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    disabled={disabled ?? false}
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className={cn(
                        "w-[var(--radix-popper-anchor-width)] justify-between",
                        className
                    )}>
                    {value
                        ? records?.find((record) => record.value === value)?.label
                        : "Select..."}
                    <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[var(--radix-popper-anchor-width)] p-0">
                <Command className={className}>
                    <CommandInput placeholder="Search..." />
                    <CommandEmpty>No values found.</CommandEmpty>
                    <CommandList>
                        <CommandGroup>
                            {records?.map((record) => (
                                <CommandItem
                                    keywords={records.map(record => record.label)}
                                    key={record.value}
                                    value={record.value}
                                    onSelect={(currentValue) => {
                                        setValue(currentValue === value ? "" : currentValue)
                                        setOpen(false)
                                    }}
                                >
                                    <Check
                                        className={cn(
                                            "mr-2 h-4 w-4",
                                            value === record.value ? "opacity-100" : "opacity-0"
                                        )}
                                    />
                                    {record.label}
                                </CommandItem>
                            ))}
                        </CommandGroup>
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}

export default Combobox