
// Ruta para manejar la generación de nómina
routerAdd("POST", "/api/payroll/employee", async (e) => {
    try {
      // Parsear y validar el cuerpo de la solicitud
      const body = e.requestInfo().body;
      const { IdEmpresa, IdMes, Empleados } = body;
  
      if (!IdEmpresa || !IdMes || !Array.isArray(Empleados)) {
        return e.json(400, { error: "Datos inválidos en la solicitud." });
      }
  
      // Buscar registros existentes de nómina
      const existingRecords = await fetchPayrollRecords(IdMes, IdEmpresa);
      const existingPayroll = existingRecords.map((record) =>
        record.publicExport()
      );
  
      // Crear nuevos registros de nómina para empleados sin registros existentes
      const newPayrollRecords = generateNewPayrollRecords(
        Empleados,
        existingPayroll,
        IdMes,
        IdEmpresa
      );
  
      if (existingRecords.length === 0 && newPayrollRecords.length === 0) {
        return e.json(200, {
          changes: false,
          message:
            "No se encontraron registros y no hay empleados nuevos para procesar.",
          payrollRecords: [],
        });
      }
  
      // Guardar nuevos registros en la base de datos
      const collection = $app.findCollectionByNameOrId("EMPLOYEE_PAYROLL");
      await savePayrollRecords(newPayrollRecords, collection);
  
      // Obtener los registros de nómina después de guardar
      const newPayrollRecordsAfterSave = await fetchPayrollRecords(
        IdMes,
        IdEmpresa
      );
      newPayrollRecordsAfterSave.forEach((record) => {
        $app.expandRecord(record, ["EMPLOYEE", "COMPANY", "MONTH"], null);
      });
  
      const newPayrollRecordsAfterSavePublicExport =
        newPayrollRecordsAfterSave.map((record) => record.publicExport());
  
      if (newPayrollRecords.length === 0) {
        // Expandir los registros existentes si no se generaron nuevos
        existingRecords.forEach((record) => {
          $app.expandRecord(record, ["EMPLOYEE", "COMPANY", "MONTH"], null);
        });
        const existingPayrollAux = existingRecords.map((record) =>
          record.publicExport()
        );
  
        return e.json(200, {
          changes: false,
          message: "No se generaron nuevos registros de nómina.",
          payrollRecords: existingPayrollAux,
        });
      }
  
      return e.json(200, {
        changes: true,
        message: "Nómina procesada exitosamente.",
        payrollRecords: newPayrollRecordsAfterSavePublicExport,
      });
    } catch (error) {
      console.error("Error al procesar nómina:", error);
      return e.json(500, { error: "Error interno del servidor." });
    }
  
    // Función para buscar registros existentes
    async function fetchPayrollRecords(month, company) {
      try {
        return await $app.findRecordsByFilter(
          "EMPLOYEE_PAYROLL",
          "MONTH = {:month} && COMPANY = {:company}",
          "-id",
          1000,
          0,
          { month, company }
        );
      } catch (error) {
        console.error("Error al obtener registros existentes:", error);
        return [];
      }
    }
  
    function calculateStartAndEndDate(monthId) {
      //get month , the variable has the id of the month
      try {
        const month = $app.findRecordById("MONTHS", monthId);
        let monthRecord = month.publicExport();
        let monthNumber = monthRecord.NUMBER;
  
        const startDate = new Date(new Date().getFullYear(), monthNumber - 1, 1);
        const endDate = new Date(new Date().getFullYear(), monthNumber, 0);
  
        return { startDate, endDate };
      } catch (error) {
        console.error("Error al obtener el mes:", error);
      }
    }
  
    function getYearsAndBasicSalary(employees) {
      let employeesData = [];
      employees.forEach((employee) => {
        try {
          const employeeRecord = $app.findRecordById("EMPLOYEE", employee);
          let employeeData = employeeRecord.publicExport();
          let dateIncorporation = new Date(employeeData.INCORPORATION_DATE);
          let basicSalaryAux = employeeData.BASIC_PAYMENT;
  
          console.log("Fecha de incorporación:", dateIncorporation);
          console.log("Salario básico:", basicSalaryAux);
  
          // Calculate years of service
          let yearsOfService = 0;
          if (dateIncorporation) {
            const now = new Date();
            yearsOfService = now.getFullYear() - dateIncorporation.getFullYear();
            if (
              now.getMonth() < dateIncorporation.getMonth() ||
              (now.getMonth() === dateIncorporation.getMonth() &&
                now.getDate() < dateIncorporation.getDate())
            ) {
              yearsOfService -= 1; // Adjust for incomplete year
            }
          }
  
          employeesData.push({
            employeeId: employee, // Include the employee ID for easy mapping
            dateIncorporation,
            basicSalaryAux,
            yearsOfService,
          });
        } catch (error) {
          console.error("Error al obtener el empleado:", error);
        }
      });
      return employeesData;
    }
  
    // Function to generate new payroll records
    function generateNewPayrollRecords(
      employees,
      existingPayroll,
      month,
      company
    ) {
      const datesObject = calculateStartAndEndDate(month);
      const employeesData = getYearsAndBasicSalary(employees);
  
      // Map employeesData to a lookup for quick access
      const employeeDataMap = Object.fromEntries(
        employeesData.map(({ employeeId, basicSalaryAux, yearsOfService }) => [
          employeeId,
          { basicSalaryAux, yearsOfService },
        ])
      );
  
      return employees
        .filter(
          (employee) =>
            !existingPayroll.some((record) => record.EMPLOYEE === employee)
        )
        .map((employee) => {
          const employeeInfo = employeeDataMap[employee] || {};
  
          return {
            EMPLOYEE: employee,
            MONTH: month,
            COMPANY: company,
            START_DATE: datesObject.startDate,
            END_DATE: datesObject.endDate,
            DAYS_PAID_MONTH: 0,
            YEARS: employeeInfo.yearsOfService || 0,
            BASIC_SALARY: employeeInfo.basicSalaryAux || 0,
            EXTRA_HOURS: 0,
            EXTRA_HOURS_BONUS_TOTAL: 0,
            HOLIDAYS: 0,
            HOLIDAYS_BONUS_TOTAL: 0,
            SUNDAYS: 0,
            SUNDAYS_BONUS_TOTAL: 0,
            DAILY_PAID_HOURS: 0,
            SENIORITY_BONUS: 0,
            TOTAL_AMOUNT_EARNED: 0,
            APORTE_RENTA_VEJEZ: 0,
            APORTE_NACIONAL_SOLIDARIO: 0,
            DESCUENTO_SIP_APORTE_SOLIDARIO_ASEGURADO: 0,
            DESCUENTO_RIESGO_COMUN: 0,
            DESCUENTO_COMISION_AFP: 0,
            TOTAL_APORTES: 0,
            RC_IVA: 0,
            TOTAL_DISCOUNTS: 0,
            NET_SALARY: 0,
            LIQUID_PAYABLE: 0,
          };
        });
    }
  
    // Función para guardar registros en la base de datos
    async function savePayrollRecords(payrollRecords, collection) {
      for (const recordData of payrollRecords) {
        try {
          const record = new Record(collection);
          Object.entries(recordData).forEach(([key, value]) =>
            record.set(key, value)
          );
          await $app.save(record);
        } catch (error) {
          console.error(
            `Error al guardar el registro para el empleado ${recordData.EMPLOYEE}:`,
            error
          );
        }
      }
    }
  });
  