# This script was generated by OpenMM-Setup on 2022-07-12.
import glob
from openmm import *
from openmm.app import *
from openmm.unit import *

def main():
    # Input Files
    pdb_files = glob.glob('inputs/protein/*.pdb')
    pdb_file_path = pdb_files[0]  # This will pick the first pdb file found
    pdb = PDBFile(pdb_file_path)
    forcefield = ForceField('amber14-all.xml', 'amber14/tip3pfb.xml')

    # System Configuration

    nonbondedMethod = PME
    nonbondedCutoff = 1.0*nanometers
    ewaldErrorTolerance = 0.0005
    constraints = HBonds
    rigidWater = True
    constraintTolerance = 0.000001
    hydrogenMass = 1.5*amu

    # Integration Options

    dt = 0.002*picoseconds
    temperature = 310*kelvin
    friction = 1.0/picosecond
    pressure = 1.0*atmospheres
    barostatInterval = 25

    # Simulation Options

    steps = 100
    equilibrationSteps = 0
    platform = Platform.getPlatformByName('CUDA')
    platformProperties = {'Precision': 'single'}
    dcdReporter = DCDReporter('outputs/trajectory.dcd', 10)
    dataReporter = StateDataReporter('outputs/log.txt', 10, totalSteps=steps,
        step=True, time=True, speed=True, progress=True, elapsedTime=True, remainingTime=True, potentialEnergy=True, kineticEnergy=True, totalEnergy=True, temperature=True, volume=True, density=True, separator='\t')
    checkpointReporter = CheckpointReporter('outputs/checkpoint.chk', 1000)

    # Prepare the Simulation

    print('Building system...')
    topology = pdb.topology
    positions = pdb.positions
    system = forcefield.createSystem(topology, nonbondedMethod=nonbondedMethod, nonbondedCutoff=nonbondedCutoff,
        constraints=constraints, rigidWater=rigidWater, ewaldErrorTolerance=ewaldErrorTolerance, hydrogenMass=hydrogenMass)
    system.addForce(MonteCarloBarostat(pressure, temperature, barostatInterval))
    integrator = LangevinMiddleIntegrator(temperature, friction, dt)
    integrator.setConstraintTolerance(constraintTolerance)
    simulation = Simulation(topology, system, integrator, platform, platformProperties)
    simulation.context.setPositions(positions)

    # Minimize and Equilibrate

    print('Performing energy minimization...')
    simulation.minimizeEnergy()
    print('Equilibrating...')
    simulation.context.setVelocitiesToTemperature(temperature)
    simulation.step(equilibrationSteps)

    # Simulate
 
    print('Simulating...')
    simulation.reporters.append(dcdReporter)
    simulation.reporters.append(dataReporter)
    simulation.reporters.append(checkpointReporter)
    simulation.currentStep = 0
    simulation.step(steps)

    # Write file with final simulation state

    state = simulation.context.getState(getPositions=True, enforcePeriodicBox=system.usesPeriodicBoundaryConditions())
    with open("outputs/final_state.pdbx", mode="w") as file:
        PDBxFile.writeFile(simulation.topology, state.getPositions(), file)
    print('Simulation complete, file written to disk as ... outputs/final_state.pdbx')

if __name__ == "__main__":
    main()
