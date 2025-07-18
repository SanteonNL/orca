import {
  IconEyeCog,
} from "@tabler/icons-react";

import { uniqueId } from "lodash";

const Menuitems = [
  {
    navlabel: true,
    subheader: "EHR",
  },
  {
    id: uniqueId(),
    title: "Patients",
    icon: IconEyeCog,
    href: "/",
  },
];

export default Menuitems;
