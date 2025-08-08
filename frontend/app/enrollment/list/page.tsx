import React from 'react'
import { Card, CardContent } from '@/components/ui/card'
import TaskOverviewTable from "@/app/enrollment/list/components/table";
import TaskHeading from "@/app/enrollment/components/task-heading";
import TaskBody from "@/app/enrollment/components/task-body";

export default function ListTasks() {
  return (
      <>
          <TaskHeading title={"Overzicht"}>
          </TaskHeading>
          <TaskBody>
              <Card className="border-0 shadow-none px-0">
                  <CardContent className="space-y-6 px-0">
                      <TaskOverviewTable></TaskOverviewTable>
                  </CardContent>
              </Card>
          </TaskBody>

      </>
  )
}

